// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otelecho // import "github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/labstack/echo/otelecho"

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/felixge/httpsnoop"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"go.opentelemetry.io/otel"

	obfuscator "github.com/helios/go-sdk/data-obfuscator"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	tracerKey  = "otel-go-contrib-tracer-labstack-echo"
	tracerName = "github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/labstack/echo/otelecho"
)

var _ io.ReadCloser = &bodyWrapper{}

// bodyWrapper wraps a http.Request.Body (an io.ReadCloser) to track the number
// of bytes read and the last error.
type bodyWrapper struct {
	io.ReadCloser
	read         int64
	err          error
	requestBody  []byte
	metadataOnly bool
}

func (w *bodyWrapper) Read(b []byte) (int, error) {
	n, err := w.ReadCloser.Read(b)
	if n > 0 {
		if !w.metadataOnly {
			w.requestBody = append(w.requestBody, b[0:n]...)
		}
	}
	n1 := int64(n)
	w.read += n1
	w.err = err
	return n, err
}

func (w *bodyWrapper) Close() error {
	return w.ReadCloser.Close()
}

type recordingResponseWriter struct {
	writer       http.ResponseWriter
	written      bool
	status       int
	responseBody []byte
	metadataOnly bool
}

var rrwPool = &sync.Pool{
	New: func() interface{} {
		return &recordingResponseWriter{}
	},
}

func getRRW(writer http.ResponseWriter) *recordingResponseWriter {
	rrw := rrwPool.Get().(*recordingResponseWriter)
	rrw.written = false
	rrw.status = 0
	rrw.responseBody = []byte{}
	rrw.writer = httpsnoop.Wrap(writer, httpsnoop.Hooks{
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(b []byte) (int, error) {
				if !rrw.written {
					rrw.written = true
					rrw.status = http.StatusOK
				}

				if !rrw.metadataOnly && len(b) > 0 {
					rrw.responseBody = append(rrw.responseBody, b...)
				}

				return next(b)
			}
		},
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(statusCode int) {
				if !rrw.written {
					rrw.written = true
					rrw.status = statusCode
				}
				next(statusCode)
			}
		},
	})
	return rrw
}

func putRRW(rrw *recordingResponseWriter) {
	rrw.writer = nil
	rrwPool.Put(rrw)
}

func collectRequestHeaders(r *http.Request, span oteltrace.Span) {
	headersStr, err := json.Marshal(r.Header)
	if err == nil {
		span.SetAttributes(attribute.KeyValue{Key: "http.request.headers", Value: attribute.StringValue(string(headersStr))})
	}
}

// Middleware returns echo middleware which will trace incoming requests.
func Middleware(service string, opts ...Option) echo.MiddlewareFunc {
	cfg := config{}
	for _, opt := range opts {
		opt.apply(&cfg)
	}
	if cfg.TracerProvider == nil {
		cfg.TracerProvider = otel.GetTracerProvider()
	}
	tracer := cfg.TracerProvider.Tracer(
		tracerName,
		oteltrace.WithInstrumentationVersion(SemVersion()),
	)
	if cfg.Propagators == nil {
		cfg.Propagators = otel.GetTextMapPropagator()
	}

	if cfg.Skipper == nil {
		cfg.Skipper = middleware.DefaultSkipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cfg.Skipper(c) {
				return next(c)
			}

			c.Set(tracerKey, tracer)
			request := c.Request()
			response := c.Response()
			savedCtx := request.Context()
			defer func() {
				request = request.WithContext(savedCtx)
				c.SetRequest(request)
				c.SetResponse(response)
			}()
			ctx := cfg.Propagators.Extract(savedCtx, propagation.HeaderCarrier(request.Header))
			opts := []oteltrace.SpanStartOption{
				oteltrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", request)...),
				oteltrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(request)...),
				oteltrace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(service, c.Path(), request)...),
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			}
			spanName := c.Path()
			if spanName == "" {
				spanName = fmt.Sprintf("HTTP %s route not found", request.Method)
			}
			metadataOnly := os.Getenv("HS_METADATA_ONLY") == "true"
			var bw bodyWrapper
			if request.Body != nil && request.Body != http.NoBody {
				bw.ReadCloser = request.Body
				bw.metadataOnly = metadataOnly
				request.Body = &bw
			}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer span.End()
			rrw := getRRW(response)
			rrw.metadataOnly = metadataOnly
			defer putRRW(rrw)

			// pass the span through the request context
			c.SetRequest(request.WithContext(ctx))
			c.SetResponse(echo.NewResponse(rrw.writer, c.Echo()))

			// serve the request to the next middleware
			err := next(c)
			if err != nil {
				span.SetAttributes(attribute.String("echo.error", err.Error()))
				// invokes the registered HTTP error handler
				c.Error(err)
			}

			attrs := semconv.HTTPAttributesFromHTTPStatusCode(c.Response().Status)
			spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(c.Response().Status, oteltrace.SpanKindServer)
			span.SetAttributes(attrs...)
			span.SetStatus(spanStatus, spanMessage)

			if !metadataOnly {
				collectRequestHeaders(request, span)
				if len(bw.requestBody) > 0 {
					attr := obfuscator.ObfuscateAttributeValue(attribute.KeyValue{Key: "http.request.body", Value: attribute.StringValue(string(bw.requestBody))})
					span.SetAttributes(attr)
				}

				if len(rrw.responseBody) > 0 {
					attr := obfuscator.ObfuscateAttributeValue(attribute.KeyValue{Key: "http.response.body", Value: attribute.StringValue(string(rrw.responseBody))})
					span.SetAttributes(attr)
				}
			}

			return nil
		}
	}
}

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

package otelmux // import "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	datautils "github.com/helios/go-sdk/data-utils"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/gorilla/mux/otelmux"
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
	contentType  string
}

func (w *bodyWrapper) Read(b []byte) (int, error) {
	n, err := w.ReadCloser.Read(b)
	if n > 0 && !w.metadataOnly{
		shouldSkipContentByType, _ := datautils.ShouldSkipContentCollectionByContentType(w.contentType)
		if !shouldSkipContentByType {
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

// Middleware sets up a handler to start tracing the incoming
// requests.  The service parameter should describe the name of the
// (virtual) server handling the request.
func Middleware(service string, opts ...Option) mux.MiddlewareFunc {
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
	if cfg.spanNameFormatter == nil {
		cfg.spanNameFormatter = defaultSpanNameFunc
	}
	return func(handler http.Handler) http.Handler {
		return traceware{
			service:           service,
			tracer:            tracer,
			propagators:       cfg.Propagators,
			handler:           handler,
			spanNameFormatter: cfg.spanNameFormatter,
		}
	}
}

func collectRequestHeaders(r *http.Request, span oteltrace.Span) {
	headersStr, err := json.Marshal(r.Header)
	if err == nil {
		span.SetAttributes(attribute.KeyValue{Key: "http.request.headers", Value: attribute.StringValue(string(headersStr))})
	}
}

type traceware struct {
	service           string
	tracer            oteltrace.Tracer
	propagators       propagation.TextMapPropagator
	handler           http.Handler
	spanNameFormatter func(string, *http.Request) string
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

func getRRW(writer http.ResponseWriter, metadataOnly bool) *recordingResponseWriter {
	rrw := rrwPool.Get().(*recordingResponseWriter)
	rrw.metadataOnly = metadataOnly
	rrw.written = false
	rrw.status = http.StatusOK
	rrw.responseBody = []byte{}
	rrw.writer = httpsnoop.Wrap(writer, httpsnoop.Hooks{
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(b []byte) (int, error) {
				if !rrw.written {
					rrw.written = true
				}
				if !rrw.metadataOnly && len(b) > 0 {
					respContentType := writer.Header().Get("Content-Type")
					shouldSkipContentByType, _ := datautils.ShouldSkipContentCollectionByContentType(respContentType)
					if !shouldSkipContentByType {
						rrw.responseBody = append(rrw.responseBody, b...)
					}
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

// defaultSpanNameFunc just reuses the route name as the span name.
func defaultSpanNameFunc(routeName string, _ *http.Request) string { return routeName }

// ServeHTTP implements the http.Handler interface. It does the actual
// tracing of the request.
func (tw traceware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := tw.propagators.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	routeStr := ""
	route := mux.CurrentRoute(r)
	if route != nil {
		var err error
		routeStr, err = route.GetPathTemplate()
		if err != nil {
			routeStr, err = route.GetPathRegexp()
			if err != nil {
				routeStr = ""
			}
		}
	}
	if routeStr == "" {
		routeStr = fmt.Sprintf("HTTP %s route not found", r.Method)
	}

	opts := []oteltrace.SpanStartOption{
		oteltrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
		oteltrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
		oteltrace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(tw.service, routeStr, r)...),
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
	}
	spanName := tw.spanNameFormatter(routeStr, r)
	metadataOnly := os.Getenv("HS_METADATA_ONLY") == "true"

	var bw bodyWrapper
	if r.Body != nil && r.Body != http.NoBody {
		bw.contentType = r.Header.Get("Content-type")
		bw.ReadCloser = r.Body
		bw.metadataOnly = metadataOnly
		r.Body = &bw
	}
	ctx, span := tw.tracer.Start(ctx, spanName, opts...)
	defer span.End()
	r2 := r.WithContext(ctx)
	rrw := getRRW(w, metadataOnly)
	defer putRRW(rrw)

	// Add traceresponse header
	if span.IsRecording() {
		spanCtx := span.SpanContext()
		rrw.writer.Header().Add("traceresponse", fmt.Sprintf("00-%s-%s-01", spanCtx.TraceID().String(), spanCtx.SpanID().String()))
	}

	tw.handler.ServeHTTP(rrw.writer, r2)
	spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(rrw.status, oteltrace.SpanKindServer)

	if !metadataOnly {
		collectRequestHeaders(r, span)
		if len(bw.requestBody) > 0 {
			attr := datautils.ObfuscateAttributeValue(attribute.KeyValue{Key: "http.request.body", Value: attribute.StringValue(string(bw.requestBody))})
			span.SetAttributes(attr)
		}

		if len(rrw.responseBody) > 0 {
			attr := datautils.ObfuscateAttributeValue(attribute.KeyValue{Key: "http.response.body", Value: attribute.StringValue(string(rrw.responseBody))})
			span.SetAttributes(attr)
		}
	}

	attrs := semconv.HTTPAttributesFromHTTPStatusCode(rrw.status)
	span.SetAttributes(attrs...)
	span.SetStatus(spanStatus, spanMessage)
}

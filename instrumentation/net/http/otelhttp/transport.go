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

package otelhttp // import "github.com/helios/opentelemetry-go-contrib/instrumentation/net/http/otelhttp"

import (
	"context"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"

	obfuscator "github.com/helios/go-sdk/data-obfuscator"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

// Transport implements the http.RoundTripper interface and wraps
// outbound HTTP(S) requests with a span.
type Transport struct {
	rt http.RoundTripper

	tracer            trace.Tracer
	propagators       propagation.TextMapPropagator
	spanStartOptions  []trace.SpanStartOption
	filters           []Filter
	spanNameFormatter func(string, *http.Request) string
	clientTrace       func(context.Context) *httptrace.ClientTrace
	metadataOnly      bool
}

var _ http.RoundTripper = &Transport{}

// NewTransport wraps the provided http.RoundTripper with one that
// starts a span and injects the span context into the outbound request headers.
//
// If the provided http.RoundTripper is nil, http.DefaultTransport will be used
// as the base http.RoundTripper.
func NewTransport(base http.RoundTripper, opts ...Option) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}

	t := Transport{
		rt:           base,
		metadataOnly: os.Getenv("HS_METADATA_ONLY") == "true",
	}

	defaultOpts := []Option{
		WithSpanOptions(trace.WithSpanKind(trace.SpanKindClient)),
		WithSpanNameFormatter(defaultTransportFormatter),
	}

	c := newConfig(append(defaultOpts, opts...)...)
	t.applyConfig(c)

	return &t
}

func (t *Transport) applyConfig(c *config) {
	t.tracer = c.Tracer
	t.propagators = c.Propagators
	t.spanStartOptions = c.SpanStartOptions
	t.filters = c.Filters
	t.spanNameFormatter = c.SpanNameFormatter
	t.clientTrace = c.ClientTrace
}

func defaultTransportFormatter(_ string, r *http.Request) string {
	return "HTTP " + r.Method
}

// RoundTrip creates a Span and propagates its context via the provided request's headers
// before handing the request to the configured base RoundTripper. The created span will
// end when the response body is closed or when a read from the body returns io.EOF.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	for _, f := range t.filters {
		if !f(r) {
			// Simply pass through to the base RoundTripper if a filter rejects the request
			return t.rt.RoundTrip(r)
		}
	}

	tracer := t.tracer

	if tracer == nil {
		if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
			tracer = newTracer(span.TracerProvider())
		} else {
			tracer = newTracer(otel.GetTracerProvider())
		}
	}

	opts := append([]trace.SpanStartOption{}, t.spanStartOptions...) // start with the configured options

	var bw bodyWrapper
	if r.Body != nil && r.Body != http.NoBody {
		bw.contentType = r.Header.Get("Content-type")
		bw.ReadCloser = r.Body
		bw.record = func(int64) {}
		bw.metadataOnly = t.metadataOnly
		r.Body = &bw
	}

	ctx, span := tracer.Start(r.Context(), t.spanNameFormatter("", r), opts...)
	if !t.metadataOnly {
		collectRequestHeaders(r, span)
	}

	if t.clientTrace != nil {
		ctx = httptrace.WithClientTrace(ctx, t.clientTrace(ctx))
	}

	r = r.WithContext(ctx)
	span.SetAttributes(semconv.HTTPClientAttributesFromHTTPRequest(r)...)
	t.propagators.Inject(ctx, propagation.HeaderCarrier(r.Header))

	res, err := t.rt.RoundTrip(r)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return res, err
	}

	if !t.metadataOnly && len(bw.requestBody) > 0 {
		attr := obfuscator.ObfuscateAttributeValue(attribute.KeyValue{Key: "http.request.body", Value: attribute.StringValue(string(bw.requestBody))})
		span.SetAttributes(attr)
	}

	span.SetAttributes(semconv.HTTPAttributesFromHTTPStatusCode(res.StatusCode)...)
	span.SetStatus(semconv.SpanStatusFromHTTPStatusCode(res.StatusCode))
	respContentType := res.Header.Get("Content-Type")
	res.Body = newWrappedBody(span, res.Body, t.metadataOnly, respContentType)

	return res, err
}

// newWrappedBody returns a new and appropriately scoped *wrappedBody as an
// io.ReadCloser. If the passed body implements io.Writer, the returned value
// will implement io.ReadWriteCloser.
func newWrappedBody(span trace.Span, body io.ReadCloser, metadataOnly bool, contentType string) io.ReadCloser {
	// The successful protocol switch responses will have a body that
	// implement an io.ReadWriteCloser. Ensure this interface type continues
	// to be satisfied if that is the case.
	if _, ok := body.(io.ReadWriteCloser); ok {
		return &wrappedBody{span: span, body: body, metadataOnly: metadataOnly}
	}

	// Remove the implementation of the io.ReadWriteCloser and only implement
	// the io.ReadCloser.
	return struct{ io.ReadCloser }{&wrappedBody{span: span, body: body, metadataOnly: metadataOnly, contentType: contentType }}
}

// wrappedBody is the response body type returned by the transport
// instrumentation to complete a span. Errors encountered when using the
// response body are recorded in span tracking the response.
//
// The span tracking the response is ended when this body is closed.
//
// If the response body implements the io.Writer interface (i.e. for
// successful protocol switches), the wrapped body also will.
type wrappedBody struct {
	span         trace.Span
	body         io.ReadCloser
	responseBody []byte
	contentType string
	metadataOnly bool
}

var _ io.ReadWriteCloser = &wrappedBody{}

func (wb *wrappedBody) Write(p []byte) (int, error) {
	// This will not panic given the guard in newWrappedBody.
	n, err := wb.body.(io.Writer).Write(p)
	if err != nil {
		wb.span.RecordError(err)
		wb.span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

func (wb *wrappedBody) Read(b []byte) (int, error) {
	n, err := wb.body.Read(b)

	if n > 0 && len(b) >= n {
		shouldSkipContentByType, _ := shouldSkipResponseContentByType(wb.contentType)
		if !wb.metadataOnly && !shouldSkipContentByType {
			wb.responseBody = append(wb.responseBody, b[0:n]...)
		}
	}

	switch err {
	case nil:
		// nothing to do here but fall through to the return
	case io.EOF:
		if !wb.metadataOnly && len(wb.responseBody) > 0 {
			attr := obfuscator.ObfuscateAttributeValue(attribute.KeyValue{Key: "http.response.body", Value: attribute.StringValue(string(wb.responseBody))})
			wb.span.SetAttributes(attr)
		}

		wb.span.End()
	default:
		wb.span.RecordError(err)
		wb.span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

func (wb *wrappedBody) Close() error {
	wb.span.End()
	if wb.body != nil {
		return wb.body.Close()
	}
	return nil
}

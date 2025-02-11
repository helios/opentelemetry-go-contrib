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

package otellambda // import "github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	obfuscator "github.com/helios/go-sdk/data-utils"
	"go.opentelemetry.io/otel/attribute"
)

// wrappedHandler is a struct which holds an instrumentor
// as well as the user's original lambda.Handler and is
// able to instrument invocations of the user's lambda.Handler.
type wrappedHandler struct {
	instrumentor instrumentor
	handler      lambda.Handler
}

// Compile time check our Handler implements lambda.Handler.
var _ lambda.Handler = wrappedHandler{}

// Invoke adds OTel span surrounding customer Handler invocation.
func (h wrappedHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := h.instrumentor.tracingBegin(ctx, payload)
	defer h.instrumentor.tracingEnd(ctx, span)

	if len(payload) > 0 {
		faasEventAttribute := obfuscator.ObfuscateAttributeValue(attribute.KeyValue{Key: "faas.event", Value: attribute.StringValue(string(payload))})
		span.SetAttributes(faasEventAttribute)
	}

	response, err := h.handler.Invoke(ctx, payload)
	if err != nil {
		return nil, err
	}

	if len(response) > 0 {
		faasResAttribute := obfuscator.ObfuscateAttributeValue(attribute.KeyValue{Key: "faas.res", Value: attribute.StringValue(string(response))})
		span.SetAttributes(faasResAttribute)
	}

	return response, nil
}

// WrapHandler Provides a Handler which wraps customer Handler with OTel Tracing.
func WrapHandler(handler lambda.Handler, options ...Option) lambda.Handler {
	return wrappedHandler{instrumentor: newInstrumentor(options...), handler: handler}
}

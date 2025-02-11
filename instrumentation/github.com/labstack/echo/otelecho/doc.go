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

// Package otelecho instruments the labstack/echo package
// (https://github.com/labstack/echo).
//
// Currently only the routing of a received message can be instrumented. To do
// so, use the Middleware function.
package otelecho // import "github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/labstack/echo/otelecho"

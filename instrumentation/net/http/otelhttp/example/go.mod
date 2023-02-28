module github.com/helios/opentelemetry-go-contrib/instrumentation/net/http/otelhttp/example

go 1.18

replace github.com/helios/opentelemetry-go-contrib/instrumentation/net/http/otelhttp => ../

require (
	github.com/helios/opentelemetry-go-contrib/instrumentation/net/http/otelhttp v0.1.0
	go.opentelemetry.io/otel v1.11.2
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v0.34.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.11.2
	go.opentelemetry.io/otel/metric v0.34.0
	go.opentelemetry.io/otel/sdk v1.11.2
	go.opentelemetry.io/otel/sdk/metric v0.34.0
	go.opentelemetry.io/otel/trace v1.11.2
)

require (
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/helios/go-sdk/data-utils v1.0.2 // indirect
	github.com/ohler55/ojg v1.17.4 // indirect
	golang.org/x/exp v0.0.0-20230203172020-98cc5a0785f9 // indirect
	golang.org/x/sys v0.1.0 // indirect
)

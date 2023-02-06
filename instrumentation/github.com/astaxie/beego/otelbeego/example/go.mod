module go.opentelemetry.io/contrib/instrumentation/github.com/astaxie/beego/otelbeego/example

go 1.18

replace (
	github.com/helios/opentelemetry-go-contrib/instrumentation/net/http/otelhttp => ../../../../../net/http/otelhttp
	go.opentelemetry.io/contrib/instrumentation/github.com/astaxie/beego/otelbeego => ../
)

require (
	github.com/astaxie/beego v1.12.3
	go.opentelemetry.io/contrib/instrumentation/github.com/astaxie/beego/otelbeego v0.37.0
	go.opentelemetry.io/otel v1.11.2
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.11.2
	go.opentelemetry.io/otel/sdk v1.11.2
	go.opentelemetry.io/otel/trace v1.11.2
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/helios/go-sdk/data-obfuscator v1.0.1 // indirect
	github.com/helios/opentelemetry-go-contrib/instrumentation/net/http/otelhttp v0.1.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/ohler55/ojg v1.17.4 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/shiena/ansicolor v0.0.0-20151119151921-a422bbe96644 // indirect
	go.opentelemetry.io/otel/metric v0.34.0 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9 // indirect
	golang.org/x/exp v0.0.0-20230203172020-98cc5a0785f9 // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.1.0 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

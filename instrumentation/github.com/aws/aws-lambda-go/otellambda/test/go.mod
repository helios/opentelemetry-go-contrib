module github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda/test

go 1.18

replace (
	github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda => ../
	go.opentelemetry.io/contrib/detectors/aws/lambda => ../../../../../../detectors/aws/lambda
	go.opentelemetry.io/contrib/propagators/aws => ../../../../../../propagators/aws
)

require (
	github.com/aws/aws-lambda-go v1.37.0
	github.com/helios/opentelemetry-go-contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda v0.37.0
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/contrib/detectors/aws/lambda v0.37.0
	go.opentelemetry.io/contrib/propagators/aws v1.12.0
	go.opentelemetry.io/otel v1.12.0
	go.opentelemetry.io/otel/sdk v1.11.2
	go.opentelemetry.io/otel/trace v1.12.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/helios/go-sdk/data-obfuscator v1.0.0 // indirect
	github.com/ohler55/ojg v1.17.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/exp v0.0.0-20230203172020-98cc5a0785f9 // indirect
	golang.org/x/sys v0.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

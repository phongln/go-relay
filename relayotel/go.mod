module github.com/phongln/go-relay/relayotel

go 1.21

require (
	github.com/phongln/go-relay v1.0.0
	go.opentelemetry.io/otel v1.26.0
	go.opentelemetry.io/otel/trace v1.26.0
)

require (
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/otel/metric v1.26.0 // indirect
)

replace github.com/phongln/go-relay => ../

module github.com/streamingfast/dgrpc

go 1.15

require (
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/prometheus/client_golang v1.12.1 // indirect
	github.com/streamingfast/dtracing v0.0.0-20210811175635-d55665d3622a
	github.com/streamingfast/logging v0.0.0-20220304214715-bc750a74b424
	github.com/streamingfast/shutter v1.5.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.uber.org/atomic v1.9.0
	go.uber.org/zap v1.21.0
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	google.golang.org/grpc v1.48.0
)

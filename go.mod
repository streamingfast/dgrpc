module github.com/dfuse-io/dgrpc

go 1.13

require (
	github.com/dfuse-io/derr master
	github.com/dfuse-io/dtracing master
	github.com/dfuse-io/logging master
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.1-0.20190717153623-606c73359dba
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/prometheus/client_golang v1.1.0 // indirect
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.2
	go.uber.org/zap v1.14.0
	google.golang.org/grpc v1.26.0
)

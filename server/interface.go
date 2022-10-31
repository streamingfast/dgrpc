package server

import (
	"time"

	gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
)

func init() {
	// https://pkg.go.dev/github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator#section-readme
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			// Putting the CloudTraceOneWayPropagator first means the TraceContext propagator
			// takes precedence if both the traceparent and the XCTC headers exist.
			gcppropagator.CloudTraceOneWayPropagator{}, // X-Cloud-Trace-Context instead of traceparent
			propagation.TraceContext{},
			propagation.Baggage{},
		))
}

type Server interface {
	RegisterService(f func(gs grpc.ServiceRegistrar))
	Launch(serverListenerAddress string)

	ServiceRegistrar() grpc.ServiceRegistrar

	OnTerminated(f func(err error))
	Shutdown(timeout time.Duration)
}

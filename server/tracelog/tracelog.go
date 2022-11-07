package tracelog

import (
	"context"
	gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/streamingfast/logging"
	sftracing "github.com/streamingfast/sf-tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
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
			RandomTraceGetter{}, // add a random traceID if there is none yet
			propagation.Baggage{},
		))
}

func SetupLoggingInterceptors(logger *zap.Logger) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unaryServerInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(withLogger(ctx, logger), req)
	}

	streamServerInterceptor := func(srv interface{}, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = withLogger(stream.Context(), logger)
		return handler(srv, wrapped)
	}
	return unaryServerInterceptor, streamServerInterceptor
}

// withLogger customizes the base logger with our own field only to reduce log cluttering
func withLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return logging.WithLogger(ctx, logger.With(zap.Stringer("trace_id", sftracing.GetTraceID(ctx))))
}

// SetupTracingInterceptors wraps the otel tracing interceptors but ensure that they contain a traceID
func SetupTracingInterceptors() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {

	unaryInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		otelUnary := otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))

		wrappedHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			if !sftracing.GetTraceID(ctx).IsValid() {
				ctx = sftracing.WithTraceID(ctx, sftracing.NewRandomTraceID()) // overwrites the whole span, unfortunately
			}
			return handler(ctx, req)
		}

		return otelUnary(ctx, req, info, wrappedHandler)
	}

	streamInterceptor := func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		otelStream := otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))

		wrappedHandler := func(srv interface{}, stream grpc.ServerStream) error {
			wrappedStream := grpc_middleware.WrapServerStream(stream)
			ctx := stream.Context()
			if !sftracing.GetTraceID(ctx).IsValid() {
				ctx = sftracing.WithTraceID(ctx, sftracing.NewRandomTraceID()) // overwrites the whole span, unfortunately
			}
			wrappedStream.WrappedContext = ctx
			return handler(srv, wrappedStream)
		}

		return otelStream(srv, stream, info, wrappedHandler)
	}

	return unaryInterceptor, streamInterceptor

}

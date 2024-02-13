package tracelog

import (
	"context"

	"connectrpc.com/connect"
	gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/streamingfast/logging"
	sftracing "github.com/streamingfast/sf-tracing"
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

func NewConnectLoggingInterceptor(logger *zap.Logger) LoggingInterceptor {
	return LoggingInterceptor{
		logger: logger,
	}
}

type LoggingInterceptor struct {
	logger *zap.Logger
}

// WrapUnary implements [Interceptor] by applying the interceptor function.
func (i LoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx = withLogger(ctx, i.logger)
		return next(ctx, req)
	}
}

// Noop
func (i LoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i LoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx = withLogger(ctx, i.logger)
		return next(ctx, conn)
	}
}

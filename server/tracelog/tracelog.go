package tracelog

import (
	"context"
	"fmt"

	gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
	connect_go "github.com/bufbuild/connect-go"
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
func (i LoggingInterceptor) WrapUnary(next connect_go.UnaryFunc) connect_go.UnaryFunc {
	return func(ctx context.Context, req connect_go.AnyRequest) (connect_go.AnyResponse, error) {
		ctx = withLogger(ctx, i.logger)
		return next(ctx, req)
	}
}

// Noop
func (i LoggingInterceptor) WrapStreamingClient(next connect_go.StreamingClientFunc) connect_go.StreamingClientFunc {
	return next
}

func (i LoggingInterceptor) WrapStreamingHandler(next connect_go.StreamingHandlerFunc) connect_go.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect_go.StreamingHandlerConn) error {
		fmt.Println("wrapped handler streaming")
		i.logger.Info("yes reawared")
		ctx = withLogger(ctx, i.logger)
		return next(ctx, conn)
	}
}

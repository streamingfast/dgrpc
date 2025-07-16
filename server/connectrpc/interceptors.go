package connectrpc

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/streamingfast/dmetrics"
	"go.uber.org/zap"
)

var connectPanicRecoveryMetrics = dmetrics.NewSet()
var connectPanicCounter = connectPanicRecoveryMetrics.NewCounterVec("connect_panic_recovered_total", []string{"handler"}, "Total number of panics recovered in Connect handlers")

func init() {
	connectPanicRecoveryMetrics.Register()
}

// IsolateRequestPanicInterceptor creates a Connect interceptor that recovers from panics
// in request handlers, preventing panics from affecting other concurrent requests.
// This interceptor logs panic details with stack traces and converts panics to proper
// Connect errors that can be handled by clients.
func IsolateRequestPanicInterceptor(logger *zap.Logger) connect.Interceptor {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					// Extract handler method name for metrics
					handlerMethod := req.Spec().Procedure

					// Record panic in metrics
					connectPanicCounter.Inc(handlerMethod)

					// Log the panic with stack trace
					logger.WithOptions(zap.AddStacktrace(zap.ErrorLevel)).Error(
						fmt.Sprintf("panic recovered in handler: %s", r),
						zap.String("procedure", handlerMethod),
					)

					// Convert panic to proper Connect error
					resp = nil
					err = connect.NewError(connect.CodeInternal, fmt.Errorf("internal server error: %v", r))
				}
			}()
			return next(ctx, req)
		})
	}

	return connect.UnaryInterceptorFunc(interceptor)
}

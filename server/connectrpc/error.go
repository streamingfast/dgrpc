package connectrpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ connect.Interceptor = (*ErrorsInterceptor)(nil)

type ErrorsInterceptor struct {
	fallbackLogger *zap.Logger
}

func NewErrorsInterceptor(fallbackLogger *zap.Logger) *ErrorsInterceptor {
	return &ErrorsInterceptor{
		fallbackLogger: fallbackLogger,
	}
}

// WrapUnary implements [Interceptor] by applying the interceptor function.
func (i *ErrorsInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err != nil {
			i.logError(ctx, req.Spec(), err)
			err = obfuscateError(err)
		}

		return resp, err
	}
}

func (i *ErrorsInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if err != nil {
			i.logError(ctx, conn.Spec(), err)
			err = obfuscateError(err)
		}

		return err
	}
}

func (i *ErrorsInterceptor) logError(ctx context.Context, spec connect.Spec, err error) {
	logger := logging.Logger(ctx, i.fallbackLogger)
	logger.Error(fmt.Sprintf("gRPC handler %s error", spec.Procedure), zap.Error(err))
}

func (i *ErrorsInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

const internalErrorMessage = "unexpected error"

func obfuscateError(err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return obfuscateConnectWebError(connectErr)
	}

	// The status conversion rules statisfies our needs (check FromError doc),
	// so we ignore if err was actually of type *status.Status
	st, _ := status.FromError(err)

	msg := st.Message()
	if st.Code() == codes.Internal || st.Code() == codes.Unknown {
		msg = internalErrorMessage
	}

	return connect.NewError(connect.Code(st.Code()), errors.New(msg))

}

func obfuscateConnectWebError(err *connect.Error) *connect.Error {
	newErr := errors.New(err.Message())
	if err.Code() == connect.CodeUnknown || err.Code() == connect.CodeInternal {
		newErr = errors.New(internalErrorMessage)
	}

	newConnectErr := connect.NewError(err.Code(), newErr)
	for _, detail := range err.Details() {
		newConnectErr.AddDetail(detail)
	}

	for key, values := range err.Meta() {
		for _, value := range values {
			newConnectErr.Meta().Add(key, value)
		}
	}

	return newConnectErr
}

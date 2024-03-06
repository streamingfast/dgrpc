package connectrpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ connect.Interceptor = (*ErrorsInterceptor)(nil)

type Option func(*ErrorsInterceptor)

func WithErrorMapper(callback func(error) error) Option {
	return func(e *ErrorsInterceptor) {
		e.errorMapper = callback
	}
}

type ErrorsInterceptor struct {
	fallbackLogger *zap.Logger
	errorMapper    func(error) error
}

func NewErrorsInterceptor(fallbackLogger *zap.Logger, options ...Option) *ErrorsInterceptor {
	e := &ErrorsInterceptor{
		fallbackLogger: fallbackLogger,
	}
	for _, opt := range options {
		opt(e)
	}
	return e
}

// WrapUnary implements [Interceptor] by applying the interceptor function.
func (i *ErrorsInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err != nil {
			connectErr := i.obfuscateError(err)
			i.logError(ctx, req.Spec(), err, connectErr)
			return resp, connectErr

		}

		return resp, err
	}
}

func (i *ErrorsInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if err != nil {
			connectErr := i.obfuscateError(err)
			i.logError(ctx, conn.Spec(), err, connectErr)
			return connectErr
		}

		return err
	}
}

func getLogSeverity(err *connect.Error) zapcore.Level {
	severity := zap.InfoLevel
	if err.Code() == connect.CodeUnknown || err.Code() == connect.CodeInternal {
		severity = zap.ErrorLevel
	}
	return severity
}

func (i *ErrorsInterceptor) logError(ctx context.Context, spec connect.Spec, cause error, err *connect.Error) {
	logger := logging.Logger(ctx, i.fallbackLogger)
	if ce := logger.Check(getLogSeverity(err), fmt.Sprintf("gRPC handler %s error", spec.Procedure)); ce != nil {
		ce.Write(zap.Error(cause))
	}
}

func (i *ErrorsInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

var InternalErrorMessage = "Unexpected error. Please try later."

func (i *ErrorsInterceptor) obfuscateError(err error) *connect.Error {
	if i.errorMapper != nil {
		err = i.errorMapper(err)
	}

	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return obfuscateConnectWebError(connectErr)
	}

	// The status conversion rules statisfies our needs (check FromError doc),
	// so we ignore if err was actually of type *status.Status
	st, _ := status.FromError(err)

	msg := st.Message()
	if st.Code() == codes.Internal || st.Code() == codes.Unknown {
		msg = InternalErrorMessage
	}

	return connect.NewError(connect.Code(st.Code()), errors.New(msg))
}

func obfuscateConnectWebError(err *connect.Error) *connect.Error {
	newErr := errors.New(err.Message())
	if err.Code() == connect.CodeUnknown || err.Code() == connect.CodeInternal {
		newErr = errors.New(InternalErrorMessage)
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

// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dgrpc

import (
	"context"
	"fmt"
	"os"

	"github.com/dfuse-io/logging"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var zlog *zap.Logger

func init() {
	logging.Register("github.com/dfuse-io/dgrpc", &zlog)

	if logger, err := setupGrpcInternalLogger(); err != nil {
		if zlog != nil {
			zlog.Warn("unable to setup internal grpc logger", zap.Error(err))
		} else {
			fmt.Fprintf(os.Stderr, "unable to setup internal grpc logger: %s", err)
		}
	} else {
		logging.Register("github.com/dfuse-io/dgrpc/internal_grpc", &logger)
	}
}

func setupGrpcInternalLogger(opts ...zap.Option) (*zap.Logger, error) {
	grpcAtomicLevel := zap.NewAtomicLevelAt(zap.ErrorLevel)
	grpcUserSpecifiedLevel := os.Getenv("GRPC_GO_ZAP_LEVEL")
	if grpcUserSpecifiedLevel != "" {
		err := grpcAtomicLevel.UnmarshalText([]byte(grpcUserSpecifiedLevel))
		if err != nil {
			return nil, err
		}
	}

	config := logging.BasicLoggingConfig("internal_grpc", grpcAtomicLevel, opts...)

	grpcInternalZlog, err := config.Build(opts...)
	if err != nil {
		return nil, err
	}

	// Level 0 verbosity in grpc-go less chatty
	// https://github.com/grpc/grpc-go/blob/master/Documentation/log_levels.md
	grpc_zap.ReplaceGrpcLoggerV2(grpcInternalZlog)
	return grpcInternalZlog, nil
}

func setupLoggingInterceptors(logger *zap.Logger) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unaryServerInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(withLogger(ctx, logger), req)
	}

	streamServerInterceptor := func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = withLogger(stream.Context(), logger)

		return handler(srv, wrapped)
	}
	return unaryServerInterceptor, streamServerInterceptor
}

func withLogger(ctx context.Context, logger *zap.Logger) context.Context {
	rootSpan := trace.FromContext(ctx)
	if rootSpan == nil {
		// to avoid this case we should call the tracing middle ware
		logger.Warn("grpc logger interceptor cannot find trace id in context")
		return ctx
	}

	traceIDField := zap.Stringer("trace_id", rootSpan.SpanContext().TraceID)

	// We customize the base logger with our own field only to reduce log cluttering
	return logging.WithLogger(ctx, logger.With(traceIDField))
}

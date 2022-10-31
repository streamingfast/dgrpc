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

package standard

import (
	"context"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/streamingfast/logging"
	sftracing "github.com/streamingfast/sf-tracing"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var zlog, _ = logging.PackageLogger("dgrpc", "github.com/streamingfast/dgrpc")
var zlogGRPC, _ = logging.PackageLogger("dgrpc", "github.com/streamingfast/dgrpc/internal_grpc")

func init() {
	// Level 0 verbosity in grpc-go less chatty
	// https://github.com/grpc/grpc-go/blob/master/Documentation/log_levels.md
	grpc_zap.ReplaceGrpcLoggerV2(zlogGRPC)
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

func withLogger(ctx context.Context, logger *zap.Logger) context.Context {
	traceID := sftracing.GetTraceID(ctx)

	// We customize the base logger with our own field only to reduce log cluttering
	return logging.WithLogger(ctx, logger.With(zap.Stringer("trace_id", traceID)))
}

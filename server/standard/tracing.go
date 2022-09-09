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
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/streamingfast/dtracing"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func SetupTracingInterceptors(logger *zap.Logger, overrideTraceID bool) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unaryServerInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// In GRPC unary calls we do not want to override the trace id of the load balancer
		return handler(withTraceID(ctx, logger, overrideTraceID), req)
	}

	// Same logic as unary interceptor, see comments there for execution flow
	streamServerInterceptor := func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// In GRPC stream we may want to override the trace id of the load balancer if we the next backend in line... (i.e dgraphql)
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = withTraceID(stream.Context(), logger, overrideTraceID)
		return handler(srv, wrapped)
	}

	return unaryServerInterceptor, streamServerInterceptor
}

func withTraceID(ctx context.Context, logger *zap.Logger, overrideTraceID bool) context.Context {
	rootTraceID := ""
	rootSpan := trace.FromContext(ctx)
	if rootSpan != nil {
		rootTraceID = rootSpan.SpanContext().TraceID.String()
	}

	// if override trace id is enabled we want to override the trace regardless if there is one or not. This should happen
	// on the user facing services, for example dgraphql
	if overrideTraceID {
		opCtx, span := dtracing.StartFreshSpan(ctx, "grpc")

		// DO NOT CHANGE THE MESSAGE LOG - FP
		logger.Info("trace_id_override",
			zap.String("root_trace_id", rootTraceID),
			zap.Stringer("trace_id", span.SpanContext().TraceID),
		)

		// We add `trace_id` to grcp_zap middleware fields, since in the middleware, those fields are added when logging the gRPC call result
		ctxzap.AddFields(opCtx, zap.Stringer("trace_id", span.SpanContext().TraceID))
		return opCtx
	}

	// We add `trace_id` to grcp_zap middleware fields, since in the middleware, those fields are added when logging the gRPC call result
	ctxzap.AddFields(ctx, zap.String("trace_id", rootTraceID))

	return ctx

}

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
	tracing "github.com/streamingfast/sf-tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var tracer = otel.Tracer("dgrpc/server/standard")

func SetupTracingInterceptors(logger *zap.Logger, overrideTraceID bool) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unaryServerInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		reqCtx, endSpan := withTraceID(ctx, logger, overrideTraceID)
		defer endSpan()

		// In GRPC unary calls we do not want to override the trace id of the load balancer
		return handler(reqCtx, req)
	}

	// Same logic as unary interceptor, see comments there for execution flow
	streamServerInterceptor := func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		reqCtx, endSpan := withTraceID(stream.Context(), logger, overrideTraceID)
		defer endSpan()

		// In GRPC stream we may want to override the trace id of the load balancer if we the next backend in line... (i.e dgraphql)
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = reqCtx

		return handler(srv, wrapped)
	}

	return unaryServerInterceptor, streamServerInterceptor
}

func withTraceID(ctx context.Context, logger *zap.Logger, overrideTraceID bool) (outCtx context.Context, cancel func()) {
	rootTraceID := ""
	if traceID := tracing.GetTraceID(ctx); traceID.IsValid() {
		rootTraceID = traceID.String()
	}

	// if override trace id is enabled we want to override the trace regardless if there is one or not. This should happen
	// on the user facing services, for example dgraphql
	if overrideTraceID {
		// Force generating a new random trace/span ID and start a new root span
		opCtx := trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: tracing.NewRandomTraceID(),
			SpanID:  tracing.NewRandomSpanID(),
		}))

		opCtx, span := tracer.Start(opCtx, "grpc", trace.WithNewRoot())
		newTraceID := tracing.GetTraceID(opCtx)

		// DO NOT CHANGE THE MESSAGE LOG - FP
		logger.Info("trace_id_override",
			zap.String("root_trace_id", rootTraceID),
			zap.Stringer("trace_id", newTraceID),
		)

		// We add `trace_id` to grcp_zap middleware fields, since in the middleware, those fields are added when logging the gRPC call result
		ctxzap.AddFields(opCtx, zap.Stringer("trace_id", newTraceID))
		return opCtx, func() { span.End() }
	}

	if rootTraceID == "" {
		randomTraceID := tracing.NewRandomTraceID()
		ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: randomTraceID,
			SpanID:  tracing.NewRandomSpanID(),
		}))
		rootTraceID = randomTraceID.String()
	}

	// We add `trace_id` to grcp_zap middleware fields, since in the middleware, those fields are added when logging the gRPC call result
	ctxzap.AddFields(ctx, zap.String("trace_id", rootTraceID))

	return ctx, func() {}
}

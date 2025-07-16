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

package server

import (
	"context"
	"runtime/debug"

	"github.com/streamingfast/dmetrics"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var panicRecoveryMetrics = dmetrics.NewSet()
var panicCounter = panicRecoveryMetrics.NewCounterVec("grpc_panic_recovered_total", []string{"handler"}, "Total number of panics recovered in gRPC handlers")

func init() {
	panicRecoveryMetrics.Register()
}

// IsolateRequestPanicUnaryInterceptor returns a new unary server interceptor that isolates panics
// to individual requests, preventing them from crashing the entire server process and affecting
// other concurrent requests. When a panic occurs, it is recovered, logged with full stack trace,
// and converted to a proper gRPC Internal error response.
func IsolateRequestPanicUnaryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Error("panic recovered in gRPC handler",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
					zap.String("stack", string(stack)),
				)

				// Record panic metric
				panicCounter.Inc(info.FullMethod)

				// Convert panic to gRPC error
				resp = nil
				err = status.Errorf(codes.Internal, "internal server error: %v", r)
			}
		}()

		resp, err = handler(ctx, req)
		return
	}
}

// IsolateRequestPanicStreamInterceptor returns a new stream server interceptor that isolates panics
// to individual streaming requests, preventing them from crashing the entire server process and affecting
// other concurrent requests. When a panic occurs, it is recovered, logged with full stack trace,
// and converted to a proper gRPC Internal error response.
func IsolateRequestPanicStreamInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Error("panic recovered in gRPC stream handler",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
					zap.String("stack", string(stack)),
				)

				// Record panic metric
				panicCounter.Inc(info.FullMethod)

				// Convert panic to gRPC error
				err = status.Errorf(codes.Internal, "internal server error: %v", r)
			}
		}()

		err = handler(srv, ss)
		return
	}
}

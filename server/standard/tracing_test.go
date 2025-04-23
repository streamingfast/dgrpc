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
	"testing"

	tracing "github.com/streamingfast/sf-tracing"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func Test_withTraceId(t *testing.T) {
	var tracer = otel.Tracer("test")

	var tests = []struct {
		name              string
		overrideTraceID   bool
		contextFunc       func() context.Context
		expectTraceIDDiff bool
	}{
		{
			name:            "Context without trace id",
			overrideTraceID: true,
			contextFunc: func() context.Context {
				return context.Background()
			},
			expectTraceIDDiff: true,
		},
		{
			name:            "with override trace id, context with trace id ",
			overrideTraceID: true,
			contextFunc: func() context.Context {
				ctx, _ := tracer.Start(context.Background(), "Testing")
				ctx = tracing.WithTraceID(ctx, tracing.NewRandomTraceID())
				return ctx
			},
			expectTraceIDDiff: true,
		},
		{
			name:            "without override trace id, context without trace id",
			overrideTraceID: false,
			contextFunc: func() context.Context {
				return context.Background()
			},
			expectTraceIDDiff: true,
		},
		{
			name:            "without override trace id, context with trace id ",
			overrideTraceID: false,
			contextFunc: func() context.Context {
				ctx, _ := tracer.Start(context.Background(), "Testing")
				ctx = tracing.WithTraceID(ctx, tracing.NewRandomTraceID())

				return ctx
			},
			expectTraceIDDiff: false,
		},
	}
	zlog := zap.NewNop()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputCtx := test.contextFunc()
			outputCtx, cancel := withTraceID(inputCtx, zlog, test.overrideTraceID)
			defer cancel()

			inputTraceID := tracing.GetTraceID(inputCtx).String()
			outputTraceID := tracing.GetTraceID(outputCtx).String()

			if test.expectTraceIDDiff {
				assert.NotEqual(t, inputTraceID, outputTraceID, "Condition %s != %s failed", inputTraceID, outputTraceID)
			} else {
				assert.Equal(t, inputTraceID, outputTraceID, "Condition %s == %s failed", inputTraceID, outputTraceID)
			}
		})
	}

}

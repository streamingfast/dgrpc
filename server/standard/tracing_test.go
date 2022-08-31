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

	"go.uber.org/zap"

	"github.com/streamingfast/dtracing"
	"github.com/stretchr/testify/assert"
)

func Test_withTraceId(t *testing.T) {

	var tests = []struct {
		name              string
		overrideTraceID   bool
		contextFunc       func() context.Context
		expectTraceIddiff bool
	}{
		{
			name:            "Context without trace id",
			overrideTraceID: true,
			contextFunc: func() context.Context {
				return context.Background()
			},
			expectTraceIddiff: true,
		},
		{
			name:            "with override trace id, context with trace id ",
			overrideTraceID: true,
			contextFunc: func() context.Context {
				ctx, _ := dtracing.StartFreshSpan(context.Background(), "Testing")
				return ctx
			},
			expectTraceIddiff: true,
		},
		{
			name:            "without override trace id, context without trace id",
			overrideTraceID: false,
			contextFunc: func() context.Context {
				return context.Background()
			},
			expectTraceIddiff: true,
		},
		{
			name:            "without override trace id, context with trace id ",
			overrideTraceID: false,
			contextFunc: func() context.Context {
				ctx, _ := dtracing.StartFreshSpan(context.Background(), "Testing")
				return ctx
			},
			expectTraceIddiff: false,
		},
	}
	zlog := zap.NewNop()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputCtx := test.contextFunc()
			outputCtx := withTraceID(inputCtx, zlog, test.overrideTraceID)

			inputTraceID := dtracing.GetTraceID(inputCtx)
			outputTraceID := dtracing.GetTraceID(outputCtx)
			if test.expectTraceIddiff {
				assert.NotEqual(t, inputTraceID, outputTraceID)
			} else {
				assert.Equal(t, inputTraceID, outputTraceID)
			}

		})
	}

}

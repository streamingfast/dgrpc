package dgrpc_test

import (
	"context"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var zlog = zap.NewNop()

var counter = atomic.Uint64{}

// See dgrpc.WithHealthCheck for rules how those returned values are used
func healthCheck(ctx context.Context) (isReady bool, out interface{}, err error) {
	count := counter.Inc()
	body := map[string]interface{}{"is_ready": true, "count": count}

	return true, body, nil
}

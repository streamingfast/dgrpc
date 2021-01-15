package dgrpc_test

import (
	"context"
	"fmt"
	"time"

	"github.com/dfuse-io/dgrpc"
	"github.com/dfuse-io/logging"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
)

var zlog = logging.NewSimpleLogger("dgrpc", "github.com/dfuse-io/dgrpc_example_secure_server")

func ExampleNewServer_Secure() {
	secureConfig, err := dgrpc.SecuredByX509KeyPair("./example/cert/cert.pem", "./example/cert/key.pem")
	if err != nil {
		panic(fmt.Errorf("unable to create X509 secure config: %w", err))
	}

	server := dgrpc.NewServer2(
		dgrpc.SecureServer(secureConfig),
		dgrpc.WithLogger(zlog),
		dgrpc.WithHealthCheck(dgrpc.HealthCheckOverGRPC, healthCheck),
	)

	server.RegisterService(func(gs *grpc.Server) {
		// Register some more gRPC services here against `gs`
		// pbstatedb.RegisterStateService(gs, implementation)
	})

	go server.Launch("localhost:9000")

	// We wait 5m before shutting down, in reality you would tie that so lifecycle of your app
	time.Sleep(5 * time.Minute)

	// Gives 30s for a gracefull shutdown
	server.Shutdown(30 * time.Second)
}

var counter = atomic.Uint64{}

// See dgrpc.WithHealthCheck for rules how those returned values are used
func healthCheck(ctx context.Context) (isReady bool, out interface{}, err error) {
	count := counter.Inc()
	body := map[string]interface{}{"is_ready": true, "count": count}

	return true, body, nil
}

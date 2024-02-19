package main

import (
	"context"
	"os"
	"time"

	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/factory"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var zlog, _ = logging.ApplicationLogger("example", "github.com/streamingfast/dgrpc/examples/plain-text-server")

func main() {
	server := factory.ServerFromOptions(
		server.WithPlainTextServer(),
		server.WithLogger(zlog),
		server.WithHealthCheck(server.HealthCheckOverHTTP|server.HealthCheckOverGRPC, healthCheck),
		server.WithRegisterService(func(gs *grpc.Server) {
			// Register some more gRPC services here against `gs`
			// pbstatedb.RegisterStateService(gs, implementation)
		}),
	)

	server.OnTerminated(func(err error) {
		if err != nil {
			zlog.Error("gRPC server unexpected failure", zap.Error(err))
		}

		// Should be tied to application lifecycle to avoid abrupt tear down
		zlog.Core().Sync()
		os.Exit(1)
	})

	go server.Launch("localhost:9000")

	// We wait 5m before shutting down, in reality you would tie that so lifecycle of your app
	time.Sleep(5 * time.Minute)

	// Gives 30s for a gracefull shutdown
	server.Shutdown(30 * time.Second)
}

func healthCheck(ctx context.Context) (isReady bool, out interface{}, err error) {
	// In your own code, you should tied the `isReady` value to the lifecycle of your application.
	// If your application is ready to accept requests, return `true`, otherwise return `false`.
	//
	// The `out` value that can anything is used by the HTTP health check (if configured in the `WithHealthCheck`
	// option by using for example `dgrpc.HealthCheckOverGRPC | dgrpc.HealthCheckOverHTTP`) will be serialized
	// in the body as JSON. It is **not** used by the GRPC health check because there is no such notion of
	// return payload.
	//
	// An error should be returned only in really rare cases, most of the time if there is an error it means
	// your application is not ready to accept requests.
	return true, nil, nil
}

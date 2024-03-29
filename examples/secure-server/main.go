package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/streamingfast/dgrpc/server/factory"

	"github.com/streamingfast/dgrpc/server"

	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var zlog, _ = logging.ApplicationLogger("example", "github.com/streamingfast/dgrpc/examples/secure-server")

func main() {
	secureConfig, err := server.SecuredByX509KeyPair("./example/cert/cert.pem", "./example/cert/key.pem")
	if err != nil {
		panic(fmt.Errorf("unable to create X509 secure config: %w", err))
	}

	srv := factory.ServerFromOptions(
		server.WithSecureServer(secureConfig),
		server.WithLogger(zlog),
		server.WithHealthCheck(server.HealthCheckOverGRPC, healthCheck),
	)

	srv.RegisterService(func(gs grpc.ServiceRegistrar) {
		// Register some more gRPC services here against `gs`
		// pbstatedb.RegisterStateService(gs, implementation)
	})

	srv.OnTerminated(func(err error) {
		if err != nil {
			zlog.Error("gRPC srv unexpected failure", zap.Error(err))
		}

		// Should be tied to application lifecycle to avoid abrupt tear down
		zlog.Core().Sync()
		os.Exit(1)
	})

	go srv.Launch("localhost:9000")

	// We wait 5m before shutting down, in reality you would tie that so lifecycle of your app
	time.Sleep(5 * time.Minute)

	// Gives 30s for a gracefull shutdown
	srv.Shutdown(30 * time.Second)
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

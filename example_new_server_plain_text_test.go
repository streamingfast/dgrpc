package dgrpc_test

import (
	"os"
	"time"

	"github.com/streamingfast/dgrpc"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func init() {
	logging.ApplicationLogger("example", "github.com/streamingfast/dgrpc_example_plain_text_server", &zlog)
}

func ExampleNewServer_PlainText() {
	server := dgrpc.NewServer2(
		dgrpc.PlainTextServer(),
		dgrpc.WithLogger(zlog),
		dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP|dgrpc.HealthCheckOverGRPC, healthCheck),
	)

	server.RegisterService(func(gs *grpc.Server) {
		// Register some more gRPC services here against `gs`
		// pbstatedb.RegisterStateService(gs, implementation)
	})

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

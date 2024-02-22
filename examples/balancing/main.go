package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/streamingfast/dgrpc"
	// You would import your own service package here
	pbacme "github.com/streamingfast/dgrpc/examples/internal/pb/acme/v1"
	"github.com/streamingfast/dgrpc/server"
	discovery_service "github.com/streamingfast/dgrpc/server/discovery-service"
	"github.com/streamingfast/dgrpc/server/factory"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var zlogClient, _ = logging.PackageLogger("client", "github.com/streamingfast/dgrpc/examples/balancing_client")
var zlogServer, _ = logging.PackageLogger("server", "github.com/streamingfast/dgrpc/examples/balancing_server")

func init() {
	logging.InstantiateLoggers(logging.WithDefaultSpec(".*=info", "google.golang.org/grpc=-"))

	// Register some extra gRPC resolvers
	dgrpc.RegisterKubernetesResolver()
}

var usage = `Usage: go run . (client <id> <remote-addr>|server <id> <listen-addr>) [<discovery-service-url>]

Runs a client or server for the ping-pong service. Run multiple on multiple IPs to see how different resolver
and balancer policies work. Dockefile can be used to build the image and run the server and client in different
containers for larger scale testing.
`

func main() {
	args := os.Args[1:]
	ensureUsage(len(args) == 3 || len(args) == 4, "Invalid number of arguments provided")

	command := args[0]
	nodeID := args[1]
	addr := args[2]

	ensureUsage(command == "client" || command == "server", "Invalid command provided, must be either 'client' or 'server'")
	ensureUsage(nodeID != "", "Argument <id> must be provided")

	if len(args) == 4 && args[3] != "" {
		configureDiscoveryService(args[3])
	}

	if command == "client" {
		runClient(nodeID, addr)
	} else {
		runServer(nodeID, addr)
	}
}

func configureDiscoveryService(discoverServiceURL string) {
	dsURL, err := url.Parse(discoverServiceURL)
	ensureNoError(err, "Invalid discovery service URL provided")

	ensureNoError(discovery_service.Bootstrap(dsURL), "Unable to bootstrap discovery service")

	if os.Getenv("GRPC_XDS_BOOTSTRAP") == "" {
		fmt.Println("You defined a discovery service URL but did not set the GRPC_XDS_BOOTSTRAP environment variable.")
		fmt.Println("This means that the discovery service will not be configured correctly. The environment variable")
		fmt.Println("should be set to a file location that is readable and writable by the current process. This file")
		fmt.Println("is created by 'github.com/streamingfast/dgrpc/server/discovery-service.Bootstrap(dsURL)' call")
		fmt.Println("which this example does.")
		os.Exit(1)
	}
}

func runClient(nodeID string, remoteAddr string) {
	logger := zlogClient.With(zap.String("node_id", nodeID))
	logger.Info("creating client", zap.String("remote_addr", remoteAddr))

	// There is a maximum of 250 streams per connection (from experience), to overcome
	// the problem, we create multiple connections and use a round-robin pool to distribute
	// the load within the client itself. This way we can lift the limit to 250 * N (assuming
	// the load is evenly distributed).
	var conns []*grpc.ClientConn
	for i := 0; i < 1; i++ {
		conn, err := dgrpc.NewInternalClientConn(remoteAddr)
		ensureNoError(err, "Unable to create client conn at %q", remoteAddr)

		conns = append(conns, conn)
	}

	conn := dgrpc.NewRoundRobinConnPool(conns)
	defer conn.Close()

	workerCount, _ := envInt("WORKER_COUNT", 10)
	msgCount, _ := envInt("MESSAGE_COUNT", 1000)

	done := make(chan string, workerCount)
	data := make(chan string, msgCount)

	client := pbacme.NewPingPongServiceClient(conn)

	for i := 0; i < workerCount; i++ {
		go func(id int) {
			defer func() {
				done <- ""
			}()

			clientID := nodeID + "/" + strconv.Itoa(id)
			logger := zlogClient.With(zap.String("node_id", clientID))

			for {
				msg, ok := <-data
				if !ok {
					logger.Info("no more client data")
					return
				}

				logger.Info("sending GetPing request", zap.String("message", msg))

				responseDelayInMillis, set := envInt("RESPONSE_DELAY_IN_MILLIS", 0)
				if !set {
					responseDelayInMillis = int(time.Duration((id * 100) + 100))
				}

				resp, err := client.GetPing(context.Background(), &pbacme.GetPingRequest{ClientId: clientID, ResponseDelayInMillis: uint64(responseDelayInMillis)})
				ensureNoError(err, "Unable to complete ping request")

				logger.Info("received GetPing response", zap.String("server_id", resp.ServerId))
			}
		}(i)
	}

	logger.Info("sending ping messages", zap.Int("count", msgCount))
	for i := 0; i < msgCount; i++ {
		data <- fmt.Sprintf("Hello message #%d", i)
		time.Sleep(10 * time.Millisecond)
	}
	close(data)

	logger.Info("waiting for worker(s) completion", zap.Int("count", workerCount))
	for i := 0; i < workerCount; i++ {
		<-done
	}

	close(done)
	logger.Info("all workers completed")
}

func runServer(nodeID string, listenAddr string) {
	server := factory.ServerFromOptions(
		server.WithPlainTextServer(),
		server.WithLogger(zlogServer),
		server.WithHealthCheck(server.HealthCheckOverGRPC, healthCheck),
		server.WithRegisterService(func(gs *grpc.Server) {
			// Here you would register your own services and implementations
			pbacme.RegisterPingPongServiceServer(gs, pbacme.NewPingPongServiceServerImpl(nodeID, zlogServer.With(zap.String("node_id", nodeID))))
		}),
	)

	server.OnTerminated(func(err error) {
		if err != nil {
			zlogServer.Error("gRPC server unexpected failure", zap.Error(err))
		}

		// Should be tied to application lifecycle to avoid abrupt tear down
		zlogServer.Core().Sync()
		os.Exit(1)
	})

	go server.Launch(listenAddr)

	signalled := make(chan os.Signal, 1)
	signal.Notify(signalled, syscall.SIGINT, syscall.SIGTERM)
	<-signalled
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

func ensureNoError(err error, msg string, args ...interface{}) {
	if err != nil {
		fmt.Println("ERROR: " + fmt.Sprintf(msg, args...))
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func ensureUsage(condition bool, msg string, args ...interface{}) {
	if !condition {
		fmt.Println("ERROR: " + fmt.Sprintf(msg, args...) + "\n\n" + usage)
		os.Exit(1)
	}
}

func envInt(key string, defaultValue int) (int, bool) {
	val, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil {
		return defaultValue, false
	}
	return int(val), true
}

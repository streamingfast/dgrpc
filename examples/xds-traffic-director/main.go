package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/streamingfast/dgrpc"
	pbacme "github.com/streamingfast/dgrpc/examples/pb/acme/v1"
	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/factory"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var zlog, _ = logging.PackageLogger("main", "github.com/streamingfast/dgrpc/examples/xds-traffic-director_main")
var zlogGRPC, _ = logging.PackageLogger("grpc", "github.com/streamingfast/dgrpc/examples/xds-traffic-director_grpc")
var zlogClient, _ = logging.PackageLogger("client", "github.com/streamingfast/dgrpc/examples/xds-traffic-director_client")
var zlogServer, _ = logging.PackageLogger("server", "github.com/streamingfast/dgrpc/examples/xds-traffic-director_server")

func init() {
	logging.InstantiateLoggers(logging.WithDefaultSpec(".*=info", "github.com/streamingfast/dgrpc/examples/xds-traffic-director_grpc=warn"))
	zap.RedirectStdLogAt(zlog, zap.InfoLevel)
	grpc_zap.ReplaceGrpcLoggerV2WithVerbosity(zlogGRPC, 0)

}

func main() {
	args := os.Args[1:]
	ensureUsage(len(args) == 4, "Invalid number of arguments provided")

	command := args[0]
	ensureUsage(command == "client" || command == "server", "Invalid command provided, must be either 'client' or 'server'")

	nodeID := args[1]
	ensureUsage(nodeID != "", "Argument <id> must be provided")

	discoveryServiceURL := args[2]
	ensureUsage(discoveryServiceURL != "", "Argument <discovery-service-url> must be provided")

	if command == "client" {
		ensureUsage(args[3] != "", "Argument <remote-addr> must be provided when running client")
		runClient(nodeID, args[3])
	} else {
		ensureUsage(args[3] != "", "Argument <listen-addr> must be provided when running server")
		runServer(nodeID, args[3])
	}
}

func runClient(nodeID string, remoteAddr string) {
	logger := zlogClient.With(zap.String("node_id", nodeID))
	logger.Info("creating client", zap.String("remote_addr", remoteAddr))

	conn, err := dgrpc.NewInternalClientConn(remoteAddr)
	ensureNoError(err, "Unable to create client conn at %q", remoteAddr)
	defer conn.Close()

	workerCount := 10
	msgCount := 1000

	done := make(chan string, workerCount)
	data := make(chan string, msgCount)

	for i := 0; i < workerCount; i++ {
		go func(id int) {
			defer func() {
				done <- ""
			}()

			client := pbacme.NewPingPongClient(conn)
			clientID := nodeID + "/" + strconv.Itoa(id)
			logger := zlogClient.With(zap.String("node_id", clientID))

			for {
				msg, ok := <-data
				if !ok {
					logger.Info("no more client data")
					return
				}

				logger.Info("sending GetPing request", zap.String("message", msg))
				resp, err := client.GetPing(context.Background(), &pbacme.GetPingRequest{ClientId: clientID, ResponseDelayInMillis: uint64(time.Duration((id * 100) + 100))})
				ensureNoError(err, "Unable to complete ping request")

				logger.Info("received GetPing response", zap.String("server_id", resp.ServerId))
			}
		}(i)
	}

	logger.Info("sending ping messages", zap.Int("count", msgCount))
	for i := 0; i < msgCount; i++ {
		data <- fmt.Sprintf("Hello message #%d", i)
	}

	logger.Info("waiting for worker(s) completion", zap.Int("count", workerCount))
	for i := 0; i < workerCount; i++ {
		<-done
	}

	close(done)
	close(data)

	logger.Info("all workers completed")
}

func runServer(nodeID string, listenAddr string) {
	server := factory.ServerFromOptions(
		server.WithPlainTextServer(),
		server.WithLogger(zlog),
		server.WithHealthCheck(server.HealthCheckOverHTTP|server.HealthCheckOverGRPC, healthCheck),
		server.WithRegisterService(func(gs *grpc.Server) {
			pbacme.RegisterPingPongServer(gs, &pingPongServer{nodeID: nodeID, logger: zlogServer.With(zap.String("node_id", nodeID))})
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

	go server.Launch(listenAddr)

	signalled := make(chan os.Signal, 1)
	signal.Notify(signalled, syscall.SIGINT, syscall.SIGTERM)
	<-signalled
}

var _ pbacme.PingPongServer = &pingPongServer{}

type pingPongServer struct {
	pbacme.UnimplementedPingPongServer
	nodeID string
	logger *zap.Logger
}

// GetPing implements pbacme.PingPongServer.
func (p *pingPongServer) GetPing(ctx context.Context, req *pbacme.GetPingRequest) (*pbacme.PingResponse, error) {
	delay := req.GetResponseDelay()

	p.logger.Info("GetPing request", zap.String("from", req.ClientId), zap.Duration("response_delay", delay), zap.String("message", req.GetMessage()))
	defer p.logger.Info("GetPing response", zap.String("from", req.ClientId))

	if int(delay) > 0 {
		time.Sleep(delay)
	}

	return &pbacme.PingResponse{
		ServerId: p.nodeID,
		Message:  fmt.Sprintf("Pong from %s (message from %s - %q)", p.nodeID, req.ClientId, req.GetMessage()),
	}, nil
}

// StreamPing implements pbacme.PingPongServer.
func (p *pingPongServer) StreamPing(req *pbacme.StreamPingRequest, sender pbacme.PingPong_StreamPingServer) error {
	delay := req.GetResponseDelay()
	if int(delay) <= 0 {
		delay = 250 * time.Millisecond
	}

	terminatesAfter := req.GetTerminatesAfter()
	if int(terminatesAfter) != 0 {
		terminatesAfter = time.Duration(math.MaxInt64)
	}

	p.logger.Info("StreamPing request", zap.String("from", req.ClientId), zap.Duration("response_delay", delay), zap.Duration("terminates_after", terminatesAfter), zap.String("message", req.GetMessage()))
	defer p.logger.Info("StreamPing terminated", zap.String("from", req.ClientId))

	terminatesTimer := time.NewTimer(terminatesAfter)
	defer terminatesTimer.Stop()

	for {
		select {
		case <-sender.Context().Done():
			p.logger.Info("StreamPing context done", zap.NamedError("err", context.Cause(sender.Context())))
			return nil
		case <-terminatesTimer.C:
			p.logger.Info("StreamPing terminated from after delay")
			return nil
		default:
			if int(delay) > 0 {
				time.Sleep(delay)
			}

			sender.SendMsg(&pbacme.PingResponse{
				ServerId: p.nodeID,
				Message:  fmt.Sprintf("Pong from %s (message from %s - %q)", p.nodeID, req.ClientId, req.GetMessage()),
			})
		}
	}
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
		fmt.Println("ERROR: " + fmt.Sprintf(msg, args...) + "\n\n" + "Usage: go run . (client <id> <discovery-service-url> <remote-addr>|server <id> <discovery-service-url> <listen-addr>)")
		os.Exit(1)
	}
}

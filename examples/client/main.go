package main

import (
	"context"
	"time"

	"github.com/streamingfast/cli"
	"github.com/streamingfast/dgrpc"
	pbacme "github.com/streamingfast/dgrpc/examples/internal/pb/acme/v1"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var zlog, _ = logging.ApplicationLogger("example", "github.com/streamingfast/dgrpc/examples/client")

func main() {
	zlog.Info("client dgrpc example starting")
	zlog.Info("ensure the server is running using 'go run ./examples/balancing server s1 localhost:9000'")

	// Show case creating a client connection to a server running on localhost:9000 with auto transport credentials.
	// This enables easy configuration of transport security based on 3 parameters, check the method signature for more details.
	connection, err := dgrpc.NewClientConn("localhost:9000", dgrpc.WithAutoTransportCredentials(false, true, false))
	cli.NoError(err, "unable to create external client")
	defer func() {
		if err := connection.Close(); err != nil {
			zlog.Error("unable to close connection gracefully", zap.Error(err))
		}
	}()

	client := pbacme.NewPingPongServiceClient(connection)

	// It's best practice to use a context with a timeout to prevent the client from blocking indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	zlog.Info("sending ping request")
	response, err := client.GetPing(ctx, &pbacme.GetPingRequest{
		ClientId:              "client-1",
		Message:               "external client dgrpc example message",
		ResponseDelayInMillis: 250,
	})
	cli.NoError(err, "unable to get ping response")

	zlog.Info("response", zap.String("message", response.Message))
}

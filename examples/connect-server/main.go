package main

import (
	"cmp"
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/streamingfast/dgrpc/examples/internal/impl"
	"github.com/streamingfast/dgrpc/examples/internal/pb/acme/v1/pbacmeconnect"
	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/connectrpc"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var zlog, _ = logging.ApplicationLogger("example", "github.com/streamingfast/dgrpc/examples/connect-server")

func main() {
	// Create Connect service implementation
	implementation := impl.NewPingPongConnectServer("connect-server-1", zlog)

	// Create handler getters for Connect services
	handlerGetters := []connectrpc.HandlerGetter{
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return pbacmeconnect.NewPingPongServiceHandler(implementation, opts...)
		},
	}

	// LISTEN_ADDR sets the listen address, plain-text by default, a trailing `*` switches to TLS using
	// the built-in self-signed (snake oil) certificate, e.g. `:8080*`
	listenAddr := cmp.Or(os.Getenv("LISTEN_ADDR"), "localhost:8080")
	listenAddr, insecureTLS := strings.CutSuffix(listenAddr, "*")
	tlsOption := server.WithPlainTextServer()
	if insecureTLS {
		tlsOption = server.WithInsecureServer()
	}

	// Create Connect server
	srv := connectrpc.New(
		handlerGetters,
		tlsOption,
		server.WithLogger(zlog),
		server.WithHealthCheck(server.HealthCheckOverHTTP, healthCheck),
		server.WithConnectPermissiveCORS(),
		server.WithConnectReflection(pbacmeconnect.PingPongServiceName),
	)

	srv.OnTerminated(func(err error) {
		if err != nil {
			zlog.Error("Connect server unexpected failure", zap.Error(err))
		}

		zlog.Core().Sync()
		os.Exit(1)
	})

	zlog.Info("starting Connect server", zap.String("listen_addr", listenAddr))
	zlog.Info("gRPC-Web example: curl -H 'Content-Type: application/json' -d '{\"clientId\":\"test\",\"message\":\"hello\"}' http://localhost:8080/acme.v1.PingPongService/GetPing")
	zlog.Info("Connect example: curl -H 'Content-Type: application/json' -d '{\"clientId\":\"test\",\"message\":\"hello\"}' http://localhost:8080/acme.v1.PingPongService/GetPing")
	zlog.Info("health check: curl http://localhost:8080/healthz")

	go srv.Launch(listenAddr)

	// We wait 5m before shutting down, in reality you would tie that to lifecycle of your app
	time.Sleep(5 * time.Minute)

	// Gives 30s for a graceful shutdown
	srv.Shutdown(30 * time.Second)
}

func healthCheck(ctx context.Context) (isReady bool, out interface{}, err error) {
	// In your own code, you should tie the `isReady` value to the lifecycle of your application.
	// If your application is ready to accept requests, return `true`, otherwise return `false`.
	return true, nil, nil
}

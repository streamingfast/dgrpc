package traffic_director

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/tracelog"
	"github.com/streamingfast/shutter"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	xdscreds "google.golang.org/grpc/credentials/xds"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/xds"
)

type TrafficDirectorServer struct {
	shutter *shutter.Shutter
	//healthServer *grpc.Server
	xdsServer *xds.GRPCServer
	logger    *zap.Logger
}

func NewServer(options *server.Options) *TrafficDirectorServer {
	logger := options.Logger
	if logger == nil {
		logger = zlog
	}

	u := options.ServiceDiscoveryURL
	useXDSCreds := strings.ToUpper(u.Query().Get("use_xds_creds")) == "TRUE"
	logger.Info("creating traffic director server", zap.Stringer("url", u), zap.Bool("use_xds_creds", useXDSCreds))

	creds := insecure.NewCredentials()
	if useXDSCreds {
		var err error
		if creds, err = xdscreds.NewServerCredentials(xdscreds.ServerOptions{FallbackCreds: insecure.NewCredentials()}); err != nil {
			log.Fatalf("failed to create trafficDirectorServer-side xDS credentials: %v", err)
		}
	}

	tracerProvider := otel.GetTracerProvider()

	// Adds contextualized logger to interceptors, must comes after authenticator since we extract stuff from there is available.
	// The interceptor tries to extract the `trace_id` from the logger and configure the logger to always use it.
	unaryLog, streamLog := tracelog.SetupLoggingInterceptors(options.Logger)

	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_prometheus.UnaryServerInterceptor,
		otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(tracerProvider)),
		unaryLog,
	}
	streamInterceptors := []grpc.StreamServerInterceptor{
		grpc_prometheus.StreamServerInterceptor,
		otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(tracerProvider)),
		streamLog,
	}

	// Adds custom defined interceptors, they come after all others
	if len(options.PostUnaryInterceptors) > 0 {
		unaryInterceptors = append(unaryInterceptors, options.PostUnaryInterceptors...)
	}

	if len(options.PostStreamInterceptors) > 0 {
		streamInterceptors = append(streamInterceptors, options.PostStreamInterceptors...)
	}

	grpcXDSServer := xds.NewGRPCServer(
		grpc.Creds(creds),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             15 * time.Second,
				PermitWithoutStream: true,
			}),
		grpc.KeepaliveParams(
			keepalive.ServerParameters{
				Time:    30 * time.Second, // Ping the client if it is idle for this amount of time
				Timeout: 10 * time.Second, // Wait this amount of time after the ping before assuming connection is dead
			}),
		grpc_middleware.WithUnaryServerChain(unaryInterceptors...),
		grpc_middleware.WithStreamServerChain(streamInterceptors...),
	)

	//healthGrpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	//healthgrpc.RegisterHealthServer(healthGrpcServer, healthServer)
	healthgrpc.RegisterHealthServer(grpcXDSServer, healthServer)

	srv := &TrafficDirectorServer{
		shutter:   shutter.New(),
		xdsServer: grpcXDSServer,
		//healthServer: healthGrpcServer,
		logger: options.Logger,
	}

	return srv
}

func (s *TrafficDirectorServer) ServiceRegistrar() grpc.ServiceRegistrar {
	return s.xdsServer
}
func (s *TrafficDirectorServer) RegisterService(f func(gs grpc.ServiceRegistrar)) {
	f(s.xdsServer)
}

// func (s *TrafficDirectorServer) Launch(serverListenerAddress string, healthServerListenerAddress string) {
func (s *TrafficDirectorServer) Launch(serverListenerAddress string) {
	serverListener, err := net.Listen("tcp4", serverListenerAddress)
	if err != nil {
		s.shutter.Shutdown(fmt.Errorf("tcp4 listening to %q: %w", serverListenerAddress, err))
		return
	}

	//healtServerListner, err := net.Listen("tcp4", healthServerListenerAddress)
	//if err != nil {
	//	s.shutter.Shutdown(fmt.Errorf("tcp4 listening to %q: %w", serverListenerAddress, err))
	//	return
	//}

	//go func() {
	err = s.xdsServer.Serve(serverListener)
	s.shutter.Shutdown(fmt.Errorf("trafficDirectorServer terminated : %w", err))
	//}()

	//err = s.healthServer.Serve(healtServerListner)
	//s.shutter.Shutdown(fmt.Errorf("healt trafficDirectorServer terminated : %w", err))
}

func (s *TrafficDirectorServer) OnTerminated(f func(err error)) {
	s.shutter.OnTerminated(f)
}

func (s *TrafficDirectorServer) Shutdown(timeout time.Duration) {
	if s.xdsServer == nil {
		return
	}

	if timeout == 0 {
		s.logger.Info("forcing gRPC trafficDirectorServer to stop")
		s.xdsServer.Stop()
		return
	}

	stopped := make(chan bool)

	// Stop the trafficDirectorServer gracefully
	go func() {
		s.logger.Info("gracefully stopping the gRPC trafficDirectorServer", zap.Duration("timeout", timeout))
		s.xdsServer.GracefulStop()
		close(stopped)
		//s.healthServer.Stop()
	}()

	// And don't wait more than 60 seconds for graceful stop to happen
	select {
	case <-time.After(timeout):
		s.logger.Info("gRPC trafficDirectorServer did not terminate gracefully within allowed time, forcing shutdown")
		s.xdsServer.Stop()
		//s.healthServer.Stop()
	case <-stopped:
		s.logger.Info("gRPC trafficDirectorServer teminated gracefully")
	}
}

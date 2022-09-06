package traffic_director

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/streamingfast/dgrpc"
	"github.com/streamingfast/shutter"
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

type ServerOption func(*serverOptions)

func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) ServerOption {
	return func(options *serverOptions) {
		options.unaryInterceptors = append(options.unaryInterceptors, interceptor)
	}
}

func WithStreamInterceptor(interceptor grpc.StreamServerInterceptor) ServerOption {
	return func(options *serverOptions) {
		options.streamInterceptors = append(options.streamInterceptors, interceptor)
	}
}
func WithAuthChecker(authChecker dgrpc.AuthCheckerFunc, enforced bool) ServerOption {
	return func(options *serverOptions) {
		options.authCheckerFunc = authChecker
		options.authCheckerEnforced = enforced
	}
}

type serverOptions struct {
	authCheckerFunc     dgrpc.AuthCheckerFunc
	authCheckerEnforced bool
	logger              *zap.Logger
	isPlainText         bool
	unaryInterceptors   []grpc.UnaryServerInterceptor
	streamInterceptors  []grpc.StreamServerInterceptor
	registrator         func(gs *grpc.Server)
	secureTLSConfig     *tls.Config
	overrideTraceID     bool
}

func newServerOptions(logger *zap.Logger) *serverOptions {
	return &serverOptions{
		logger:          logger,
		isPlainText:     true,
		overrideTraceID: false,
	}
}

type Server struct {
	shutter      *shutter.Shutter
	healthServer *grpc.Server
	xdsServer    *xds.GRPCServer
	logger       *zap.Logger
}

func NewServer(u *url.URL, logger *zap.Logger, opts ...ServerOption) *Server {
	options := newServerOptions(logger)
	for _, opt := range opts {
		opt(options)
	}

	if options.authCheckerFunc != nil {
		options.unaryInterceptors = append(options.unaryInterceptors, dgrpc.UnaryAuthChecker(options.authCheckerEnforced, options.authCheckerFunc))
		options.streamInterceptors = append(options.streamInterceptors, dgrpc.StreamAuthChecker(options.authCheckerEnforced, options.authCheckerFunc))
	}

	useXDSCreds := strings.ToUpper(u.Query().Get("use-xds-reds")) == "TRUE"

	creds := insecure.NewCredentials()
	if useXDSCreds {
		log.Println("Using xDS credentials...")
		var err error
		if creds, err = xdscreds.NewServerCredentials(xdscreds.ServerOptions{FallbackCreds: insecure.NewCredentials()}); err != nil {
			log.Fatalf("failed to create server-side xDS credentials: %v", err)
		}
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
		grpc_middleware.WithStreamServerChain(options.streamInterceptors...),
		grpc_middleware.WithUnaryServerChain(options.unaryInterceptors...),
	)

	healthGrpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthgrpc.RegisterHealthServer(healthGrpcServer, healthServer)

	srv := &Server{
		shutter:      shutter.New(),
		xdsServer:    grpcXDSServer,
		healthServer: healthGrpcServer,
		logger:       logger,
	}

	return srv
}
func (s *Server) RegisterService(f func(gs *xds.GRPCServer)) {
	f(s.xdsServer)
}

func (s *Server) Launch(serverListenerAddress string, healthServerListenerAddress string) {
	serverListener, err := net.Listen("tcp4", serverListenerAddress)
	if err != nil {
		s.shutter.Shutdown(fmt.Errorf("tcp4 listening to %q: %w", serverListenerAddress, err))
		return
	}

	healtServerListner, err := net.Listen("tcp4", healthServerListenerAddress)
	if err != nil {
		s.shutter.Shutdown(fmt.Errorf("tcp4 listening to %q: %w", serverListenerAddress, err))
		return
	}

	go func() {
		err := s.xdsServer.Serve(serverListener)
		s.shutter.Shutdown(fmt.Errorf("server terminated : %w", err))
	}()

	err = s.healthServer.Serve(healtServerListner)
	s.shutter.Shutdown(fmt.Errorf("healt server terminated : %w", err))
}

func (s *Server) OnTerminated(f func(err error)) {
	s.shutter.OnTerminated(f)
}

func (s *Server) Shutdown(timeout time.Duration) {
	if s.xdsServer == nil {
		return
	}

	if timeout == 0 {
		s.logger.Info("forcing gRPC server to stop")
		s.xdsServer.Stop()
		return
	}

	stopped := make(chan bool)

	// Stop the server gracefully
	go func() {
		s.logger.Info("gracefully stopping the gRPC server", zap.Duration("timeout", timeout))
		s.xdsServer.GracefulStop()
		close(stopped)
		s.healthServer.Stop()
	}()

	// And don't wait more than 60 seconds for graceful stop to happen
	select {
	case <-time.After(timeout):
		s.logger.Info("gRPC server did not terminate gracefully within allowed time, forcing shutdown")
		s.xdsServer.Stop()
		s.healthServer.Stop()
	case <-stopped:
		s.logger.Info("gRPC server teminated gracefully")
	}
}

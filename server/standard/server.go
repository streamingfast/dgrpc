// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package standard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/tracelog"
	"github.com/streamingfast/shutter"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	pbhealth "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

var readyResponse = map[string]interface{}{"is_ready": true}
var notReadyResponse = map[string]interface{}{"is_ready": false}

// Verbosity is configuration that can be used to globally reduce
// logging chattiness of various aspect of the dgrpc middleware.
//
// Accepted value is a scale from 0 to 5 (inclusively). A verbosity of 0
// means really not verbose, while 5 means really really verbose. It can
// be mostly seen as: `Fatal` (0), `Error` (1), `Warn` (2), `Info` (3),
// `Debug` (4) and `Trace` (5).
//
// For now, this controls server logging of gRCP code to zap level which
// reduce some of the case into `INFO` level and some more like `OK` on `DEBUG`
// level.
var Verbosity = 3

// StandardServer is actually a thin wrapper struct around an `*http.StandardServer` and a
// `*grpc.StandardServer` to more easily managed how we deploy our gRPC service.
//
// You first create a NewServer2(...) with the various option we provide for
// how stuff should be wired together. You then `Launch` it and the wrapper
// takes care of doing the hard code of correctly managing the lifecyle of
// everything.
//
// Here how various options affects the behavior of the `Launch` method.
// - WithSecure(SecuredByX509KeyPair(..., ...)) => Starts a TLS HTTP2 endpoint to serve the gRPC over an encrypted connection
// - WithHealthCheck(check, HealthCheckOverHTTP) => Offers an HTTP endpoint `/healthz` to query the health check over HTTP
type StandardServer struct {
	shutter    *shutter.Shutter
	options    *server.Options
	grpcServer *grpc.Server
	httpServer *http.Server
}

// NewServer
//
// The server is easily configurable by providing various `dgrpc.Option`
// options to change the behaviour of the server.
//
// This new server is opinionated towards dfuse needs (for example supporting the
// health check over HTTP) but is generic and configurable enough to used by any
// organization.
//
// Some elements are "hard-coded" but we are willing to open more the configuration
// if requested by the community.
//
// **Important** We use `NewServer2` name temporarily while we test the concept, when
//
//	we are satisfied with the interface and feature set, the actual
//	`NewServer` will be replaced by this implementation.
func NewServer(options *server.Options) *StandardServer {
	srv := &StandardServer{
		shutter:    shutter.New(),
		options:    options,
		grpcServer: newGRPCServer(options),
	}

	if options.HealthCheck != nil && server.HealthCheckOverGRPC.IsActive(uint8(options.HealthCheckOver)) {
		pbhealth.RegisterHealthServer(srv.grpcServer, srv.healthGRPCHandler())
	}

	if options.Registrator != nil {
		options.Registrator(srv.grpcServer)
	}

	return srv
}

func (s *StandardServer) GrpcServer() *grpc.Server {
	return s.grpcServer
}

// Launch starts all the necessary elements (gRPC StandardServer, HTTP StandardServer if required, etc.) and
// controls their lifecycle via the internal shutter.
//
// This should be called in a Goroutine `go server.Launch("localhost:9000")` and
// `server.Shutdown()` should be called later on to stop gracefully the server.
func (s *StandardServer) Launch(serverListenerAddress string) {
	s.logger().Info("launching gRPC server", zap.String("listen_addr", serverListenerAddress))
	tcpListener, err := net.Listen("tcp", serverListenerAddress)
	if err != nil {
		s.shutter.Shutdown(fmt.Errorf("tcp listening to %q: %w", serverListenerAddress, err))
		return
	}

	httpErrorLogger, err := zap.NewStdLogAt(s.logger(), zap.ErrorLevel)
	if err != nil {
		s.shutter.Shutdown(fmt.Errorf("unable to create logger: %w", err))
		return
	}

	h2s := &http2.Server{
		MaxConcurrentStreams: 1000,
	}

	// We start an HTTP server only when having an health check that requires HTTP transport
	if s.options.HealthCheck != nil && server.HealthCheckOverHTTP.IsActive(uint8(s.options.HealthCheckOver)) {
		healthHandler := s.HealthHandler()
		muxRoot := mux.NewRouter()

		muxRoot.Path("/").Handler(healthHandler)
		muxRoot.Path("/healthz").Handler(healthHandler)
		muxRoot.PathPrefix("/").Handler(CompressionHandler(s.options.EnforceCompression, s.grpcServer))
		s.httpServer = &http.Server{
			Handler:  h2c.NewHandler(muxRoot, h2s),
			ErrorLog: httpErrorLogger,
		}

		if s.options.SecureTLSConfig != nil {
			s.logger().Info("serving gRPC (over HTTP router) (encrypted)", zap.String("listen_addr", serverListenerAddress))
			s.httpServer.TLSConfig = s.options.SecureTLSConfig
			if err := s.httpServer.ServeTLS(tcpListener, "", ""); err != nil {
				s.shutter.Shutdown(fmt.Errorf("gRPC (over HTTP router) serve (TLS) failed: %w", err))
				return
			}
		} else if s.options.IsPlainText {
			s.logger().Info("serving gRPC (over HTTP router) (plain-text)", zap.String("listen_addr", serverListenerAddress))

			if err := s.httpServer.Serve(tcpListener); err != nil {
				s.shutter.Shutdown(fmt.Errorf("gRPC (over HTTP router) serve failed: %w", err))
				return
			}
		} else {
			s.shutter.Shutdown(errors.New("invalid server config, server is not plain-text and no TLS config available, something is wrong, this should never happen"))
			return
		}

		return
	}

	compressionHandler := CompressionHandler(s.options.EnforceCompression, s.grpcServer)
	s.httpServer = &http.Server{
		Handler:  h2c.NewHandler(compressionHandler, h2s),
		ErrorLog: httpErrorLogger,
	}

	s.logger().Info("serving gRPC", zap.String("listen_addr", serverListenerAddress))
	if err := s.httpServer.Serve(tcpListener); err != nil {
		s.shutter.Shutdown(fmt.Errorf("gRPC (over HTTP router) serve failed: %w", err))
		return
	}
}

func (s *StandardServer) healthCheck(ctx context.Context) (isReady bool, out interface{}, err error) {
	if s.shutter.IsTerminating() {
		return false, nil, nil
	}

	check := s.options.HealthCheck
	if check == nil {
		return false, nil, errors.New("trying to check health while health check is not set, this is invalid")
	}

	return check(ctx)
}

func (s *StandardServer) healthGRPCHandler() pbhealth.HealthServer {
	return server.NewHealthGRPCHandler(s.healthCheck)
}

type errorResponse struct {
	Error error `json:"error"`
}

func (s *StandardServer) logger() *zap.Logger {
	return s.options.Logger
}

func (s *StandardServer) Error() error {
	return s.shutter.Err()
}

func (s *StandardServer) Terminating() <-chan struct{} {
	return s.shutter.Terminating()
}

// RegisterService can be used to register your own gRPC service handler.
//
//	server := dgrpc.NewServer2(...)
//	server.RegisterService(func (gs *grpc.StandardServer) {
//	  pbapi.RegisterStateService(gs, implementation)
//	})
//
// **Note**
func (s *StandardServer) RegisterService(f func(gs grpc.ServiceRegistrar)) {
	f(s.grpcServer)
}
func (s *StandardServer) ServiceRegistrar() grpc.ServiceRegistrar {
	return s.grpcServer
}

func (s *StandardServer) OnTerminated(f func(err error)) {
	s.shutter.OnTerminated(f)
}

func (s *StandardServer) Shutdown(timeout time.Duration) {
	if s.httpServer != nil {
		s.shutdownViaHTTP(timeout)
	} else {
		s.shutdownViaGRPC(timeout)
	}
}

func (s *StandardServer) shutdownViaGRPC(timeout time.Duration) {
	if s.grpcServer == nil {
		return
	}

	if timeout == 0 {
		s.logger().Info("forcing gRPC server to stop")
		s.grpcServer.Stop()
		return
	}

	stopped := make(chan bool)

	// Stop the server gracefully
	go func() {
		s.logger().Info("gracefully stopping the gRPC server", zap.Duration("timeout", timeout))
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	// And don't wait more than 60 seconds for graceful stop to happen
	select {
	case <-time.After(timeout):
		s.logger().Info("gRPC server did not terminate gracefully within allowed time, forcing shutdown")
		s.grpcServer.Stop()
	case <-stopped:
		s.logger().Info("gRPC server teminated gracefully")
	}
}

func (s *StandardServer) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isReady, out, err := s.healthCheck(r.Context())

		w.Header().Set("Content-Type", "application/json")

		if !isReady {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		var body interface{}
		if out != nil && err == nil {
			body = out
		} else if err != nil {
			body = errorResponse{Error: err}
		} else if isReady {
			body = readyResponse
		} else {
			body = notReadyResponse
		}

		bodyJSON, err := json.Marshal(body)
		if err == nil {
			w.Write(bodyJSON)
		} else {
			// We were unable to marshal body to JSON, let's actually return the marshalling error now.
			// There is no reason that the below `json.Marshal` would fail here, but it it's the case, we finally give up.
			fallbackBodyJSON, err := json.Marshal(map[string]interface{}{
				"error": fmt.Errorf("unable to marshal health check body (of type %T) to JSON: %w", body, err),
			})
			if err == nil {
				w.Write(fallbackBodyJSON)
			}
		}
	})
}

func (s *StandardServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.grpcServer.ServeHTTP(rw, req)
}

func (s *StandardServer) shutdownViaHTTP(timeout time.Duration) {
	if s.httpServer == nil {
		return
	}

	ctx := context.Background()
	if timeout != 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()

		s.logger().Info("gracefully stopping the gRPC server (over HTTP router)", zap.Duration("timeout", timeout))
	} else {
		s.logger().Info("forcing gRPC server (over HTTP router) to stop")
	}

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		s.logger().Warn("gRPC server (over HTTP router) did not terminate gracefully within allowed time", zap.Error(err))
	} else {
		s.logger().Info("gRPC server (over HTTP router) terminated gracefully")
	}
}

// newGRPCServer creates a new standard fully configured with tracing, logging and
// more.
//
// **Note** Debugging a gRPC server can be done by using `export GODEBUG=http2debug=2`
func newGRPCServer(options *server.Options) *grpc.Server {
	zopts := []grpc_zap.Option{
		grpc_zap.WithDurationField(grpc_zap.DurationToTimeMillisField),
		grpc_zap.WithDecider(defaultLoggingDecider),
		grpc_zap.WithLevels(defaultServerCodeLevel),
	}

	// Zap base server interceptor
	zapUnaryInterceptor := grpc_zap.UnaryServerInterceptor(options.Logger, zopts...)
	zapStreamInterceptor := grpc_zap.StreamServerInterceptor(options.Logger, zopts...)

	tracerProvider := otel.GetTracerProvider()

	// Order of interceptors is important here, index order is followed so `{one, two, three}` runs `one` then `two` then `three` passing context along the way
	streamInterceptors := []grpc.StreamServerInterceptor{}
	unaryInterceptors := []grpc.UnaryServerInterceptor{}

	// Adds custom defined pre-interceptors first, they run before all others
	if len(options.PreUnaryInterceptors) > 0 {
		unaryInterceptors = append(unaryInterceptors, options.PreUnaryInterceptors...)
	}

	if len(options.PreStreamInterceptors) > 0 {
		streamInterceptors = append(streamInterceptors, options.PreStreamInterceptors...)
	}

	// Add default panic isolation interceptors
	streamInterceptors = append(streamInterceptors, server.IsolateRequestPanicStreamInterceptor(options.Logger))
	unaryInterceptors = append(unaryInterceptors, server.IsolateRequestPanicUnaryInterceptor(options.Logger))

	// Add standard dgrpc interceptors
	streamInterceptors = append(streamInterceptors,
		grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
		grpc_prometheus.StreamServerInterceptor,
		zapStreamInterceptor, // zap base server interceptor
	)

	unaryInterceptors = append(unaryInterceptors,
		grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
		grpc_prometheus.UnaryServerInterceptor,
		zapUnaryInterceptor, // zap base server interceptor
	)

	// Adds contextualized logger to interceptors, must comes after authenticator since we extract stuff from there is available.
	// The interceptor tries to extract the `trace_id` from the logger and configure the logger to always use it.
	unaryLog, streamLog := tracelog.SetupLoggingInterceptors(options.Logger)

	unaryInterceptors = append(unaryInterceptors, unaryLog)
	streamInterceptors = append(streamInterceptors, streamLog)

	// Adds custom defined interceptors, they come after all others
	if len(options.PostUnaryInterceptors) > 0 {
		unaryInterceptors = append(unaryInterceptors, options.PostUnaryInterceptors...)
	}

	if len(options.PostStreamInterceptors) > 0 {
		streamInterceptors = append(streamInterceptors, options.PostStreamInterceptors...)
	}

	s := grpc.NewServer(
		append([]grpc.ServerOption{
			grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithTracerProvider(tracerProvider))),
			grpc.KeepaliveEnforcementPolicy(
				keepalive.EnforcementPolicy{
					MinTime:             15 * time.Second,
					PermitWithoutStream: true,
				},
			),
			grpc.KeepaliveParams(
				keepalive.ServerParameters{
					Time:    30 * time.Second, // Ping the client if it is idle for this amount of time
					Timeout: 10 * time.Second, // Wait this amount of time after the ping before assuming connection is dead
				},
			),
			grpc_middleware.WithStreamServerChain(streamInterceptors...),
			grpc_middleware.WithUnaryServerChain(unaryInterceptors...),
		}, options.ServerOptions...)...,
	)

	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets([]float64{0, .5, 1, 2, 3, 5, 8, 10, 20, 30}))
	grpc_prometheus.Register(s)
	reflection.Register(s)

	return s
}

func defaultLoggingDecider(fullMethodName string, err error) bool {
	if err == nil && fullMethodName == "/grpc.health.v1.Health/Check" {
		return false
	}

	return true
}

func defaultServerCodeLevel(code codes.Code) zapcore.Level {
	// Our default is Verbosity 3 (which essentially means Info) and higher means more verbose
	// while lower means less verbose.
	if Verbosity <= 2 {
		return defaultServerCodeLessVerbosity(code)
	}

	return defaultServerCodeNormalVerbosity(code)
}

// defaultServerCodeNormalVerbosity maps gRPC codes to zap logging levels. Our rule of thumb
// is that it's something expected to happen, log in Debug, unusual but not alarming is Info, something
// that should be looked at is Warn and something really bad is Error.
func defaultServerCodeNormalVerbosity(code codes.Code) zapcore.Level {
	switch code {
	case codes.OK:
		return zap.DebugLevel
	case codes.Canceled:
		return zap.DebugLevel
	case codes.Unknown:
		return zap.ErrorLevel
	case codes.InvalidArgument:
		return zap.DebugLevel
	case codes.DeadlineExceeded:
		return zap.InfoLevel
	case codes.NotFound:
		return zap.DebugLevel
	case codes.AlreadyExists:
		return zap.InfoLevel
	case codes.PermissionDenied:
		return zap.DebugLevel
	case codes.Unauthenticated:
		return zap.DebugLevel // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return zap.InfoLevel
	case codes.FailedPrecondition:
		return zap.InfoLevel
	case codes.Aborted:
		return zap.InfoLevel
	case codes.OutOfRange:
		return zap.InfoLevel
	case codes.Unimplemented:
		return zap.ErrorLevel
	case codes.Internal:
		return zap.ErrorLevel
	case codes.Unavailable:
		return zap.InfoLevel
	case codes.DataLoss:
		return zap.ErrorLevel
	default:
		return zap.ErrorLevel
	}
}

func defaultServerCodeLessVerbosity(code codes.Code) zapcore.Level {
	switch code {
	case codes.OK:
		return zap.DebugLevel
	case codes.Canceled:
		return zap.DebugLevel
	case codes.Unknown:
		return zap.DebugLevel
	case codes.InvalidArgument:
		return zap.DebugLevel
	case codes.DeadlineExceeded:
		return zap.InfoLevel
	case codes.NotFound:
		return zap.DebugLevel
	case codes.AlreadyExists:
		return zap.DebugLevel
	case codes.PermissionDenied:
		return zap.InfoLevel
	case codes.Unauthenticated:
		return zap.DebugLevel // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return zap.InfoLevel
	case codes.FailedPrecondition:
		return zap.InfoLevel
	case codes.Aborted:
		return zap.InfoLevel
	case codes.OutOfRange:
		return zap.InfoLevel
	case codes.Unimplemented:
		return zap.ErrorLevel
	case codes.Internal:
		return zap.ErrorLevel
	case codes.Unavailable:
		return zap.InfoLevel
	case codes.DataLoss:
		return zap.ErrorLevel
	default:
		return zap.ErrorLevel
	}
}

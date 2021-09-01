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

package dgrpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/streamingfast/dgrpc/insecure"
	pbhealth "github.com/streamingfast/pbgo/grpc/health/v1"
	"github.com/streamingfast/shutter"
	"go.opencensus.io/plugin/ocgrpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Verbosity is configuration that can be used to globally reduce
// logging chattiness of various aspect of the dgrpc middleware.
//
// Accepted value is a scale from 0 to 5 (inclusively). A verbosity of 0
// means really not verbose, while 5 means really realy verbose. It can
// be mostly seen as: `Fatal` (0), `Error` (1), `Warn` (2), `Info` (3),
// `Debug` (4) and `Trace` (5).
//
// For now, this controls server logging of gRCP code to zap level which
// reduce some of the case into `INFO` level and some more like `OK` on `DEBUG`
// level.
var Verbosity = 3

type HealthCheck func(ctx context.Context) (isReady bool, out interface{}, err error)

type serverOptions struct {
	authCheckerFunc        AuthCheckerFunc
	authCheckerEnforced    bool
	healthCheck            HealthCheck
	healthCheckOver        HealthCheckOver
	logger                 *zap.Logger
	isPlainText            bool
	postUnaryInterceptors  []grpc.UnaryServerInterceptor
	postStreamInterceptors []grpc.StreamServerInterceptor
	registers              func(gs *grpc.Server)
	secureTLSConfig        *tls.Config
	overrideTraceID        bool
}

func newServerOptions() *serverOptions {
	return &serverOptions{
		logger:          zlog,
		isPlainText:     true,
		overrideTraceID: false,
	}
}

// Server is actually a thin wrapper struct around an `*http.Server` and a
// `*grpc.Server` to more easily managed how we deploy our gRPC service.
//
// You first create a NewServer2(...) with the various option we provide for
// how stuff should be wired together. You then `Launch` it and the wrapper
// takes care of doing the hard code of correctly managing the lifecyle of
// everything.
//
// Here how various options affects the behavior of the `Launch` method.
// - WithSecure(SecuredByX509KeyPair(..., ...)) => Starts a TLS HTTP2 endpoint to serve the gRPC over an encrypted connection
// - WithHealthCheck(check, HealthCheckOverHTTP) => Offers an HTTP endpoint `/healthz` to query the health check over HTTP
//
type Server struct {
	shutter    *shutter.Shutter
	options    *serverOptions
	grpcServer *grpc.Server
	httpServer *http.Server
}

// NewServer2 creates a new thin-wrapper `dgrpc.Server` instance to make it easy
// to configure and launch a gRPC server that has a well-defined lifecycle tied
// to `shutter.Shutter` pattern.
//
// The server is easily configurable by providing various `dgrpc.ServerOption`
// options to change the behaviour of the server.
//
// This new server is opinionated towards dfuse needs (for example supporting the
// health check over HTTP) but is generic and configurable enough to used by any
// organization.
//
// Some elements are "hard-coded" but we are willing to open more the configuration
// if requested by the community.
//
// **Important** We use `NewServer2` name temporarly while we test the concept, when
//               we are statisfied with the interface and feature set, the actual
//               `NewServer` will be replaced by this implementation.
func NewServer2(opts ...ServerOption) *Server {
	options := newServerOptions()
	for _, opt := range opts {
		opt(options)
	}

	server := &Server{
		shutter:    shutter.New(),
		options:    options,
		grpcServer: newGRPCServer(options),
	}

	if options.healthCheck != nil && HealthCheckOverGRPC.isActive(uint8(options.healthCheckOver)) {
		pbhealth.RegisterHealthServer(server.grpcServer, server.healthGRPCHandler())
	}

	return server
}

// Launch starts all the necessary elements (gRPC Server, HTTP Server if required, etc.) and
// controls their lifecycle via the internal shutter.
//
// This should be called in a Goroutine `go server.Launch("localhost:9000")` and
// `server.Shutdown()` should be called later on to stop gracefully the server.
func (s *Server) Launch(listenAddr string) {
	s.logger().Debug("launching gRPC server", zap.String("listen_addr", listenAddr))
	tcpListener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		s.shutter.Shutdown(fmt.Errorf("tcp listening to %q: %w", listenAddr, err))
		return
	}

	// We start an HTTP server only when having an health check that requires HTTP transport
	if s.options.healthCheck != nil && HealthCheckOverHTTP.isActive(uint8(s.options.healthCheckOver)) {
		healthHandler := s.healthHandler()
		grpcRouter := mux.NewRouter()

		grpcRouter.Path("/").Handler(healthHandler)
		grpcRouter.Path("/healthz").Handler(healthHandler)
		grpcRouter.PathPrefix("/").Handler(s.grpcServer)

		errorLogger, err := zap.NewStdLogAt(s.logger(), zap.ErrorLevel)
		if err != nil {
			s.shutter.Shutdown(fmt.Errorf("unable to create logger: %w", err))
			return
		}

		s.httpServer = &http.Server{
			Handler:  grpcRouter,
			ErrorLog: errorLogger,
		}

		if s.options.secureTLSConfig != nil {
			s.logger().Info("serving gRPC (over HTTP router) (encrypted)", zap.String("listen_addr", listenAddr))
			s.httpServer.TLSConfig = s.options.secureTLSConfig
			if err := s.httpServer.ServeTLS(tcpListener, "", ""); err != nil {
				s.shutter.Shutdown(fmt.Errorf("gRPC (over HTTP router) serve (TLS) failed: %w", err))
				return
			}
		} else if s.options.isPlainText {
			s.logger().Info("serving gRPC (over HTTP router) (plain-text)", zap.String("listen_addr", listenAddr))

			h2s := &http2.Server{}
			s.httpServer.Handler = h2c.NewHandler(grpcRouter, h2s)

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

	s.logger().Info("serving gRPC", zap.String("listen_addr", listenAddr))
	if err := s.grpcServer.Serve(tcpListener); err != nil {
		s.shutter.Shutdown(fmt.Errorf("gRPC serve failed: %w", err))
		return
	}
}

func (s *Server) healthCheck(ctx context.Context) (isReady bool, out interface{}, err error) {
	if s.shutter.IsTerminating() {
		return false, nil, nil
	}

	check := s.options.healthCheck
	if check == nil {
		return false, nil, errors.New("trying to check health while health check is not set, this is invalid")
	}

	return check(ctx)
}

func (s *Server) healthGRPCHandler() pbhealth.HealthServer {
	return healthGRPCHandler{check: s.healthCheck}
}

type healthGRPCHandler struct {
	check HealthCheck
}

func (c healthGRPCHandler) Check(ctx context.Context, _ *pbhealth.HealthCheckRequest) (*pbhealth.HealthCheckResponse, error) {
	isReady, _, err := c.check(ctx)
	if err != nil {
		return nil, err
	}

	status := pbhealth.HealthCheckResponse_NOT_SERVING
	if isReady {
		status = pbhealth.HealthCheckResponse_SERVING
	}

	return &pbhealth.HealthCheckResponse{Status: status}, nil
}

var readyResponse = map[string]interface{}{"is_ready": true}
var notReadyResponse = map[string]interface{}{"is_ready": false}

func (s *Server) healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isReady, out, err := s.healthCheck(r.Context())

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

type errorResponse struct {
	Error error `json:"error"`
}

func (s *Server) logger() *zap.Logger {
	return s.options.logger
}

// RegisterService can be used to register your own gRPC service handler.
//
//     server := dgrpc.NewServer2(...)
//     server.RegisterService(func (gs *grpc.Server) {
//       pbapi.RegisterStateService(gs, implementation)
//     })
//
// **Note**
func (s *Server) RegisterService(f func(gs *grpc.Server)) {
	f(s.grpcServer)
}

func (s *Server) OnTerminated(f func(err error)) {
	s.shutter.OnTerminated(f)
}

func (s *Server) Shutdown(timeout time.Duration) {
	if s.httpServer != nil {
		s.shutdownViaHTTP(timeout)
	} else {
		s.shutdownViaGRPC(timeout)
	}
}

func (s *Server) shutdownViaGRPC(timeout time.Duration) {
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

func (s *Server) shutdownViaHTTP(timeout time.Duration) {
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

// ServerOption represents option that can be used when constructing a gRPC
// server to customize its behavior.
type ServerOption func(*serverOptions)

type AuthCheckerFunc func(ctx context.Context, token, ipAddress string) (context.Context, error)

// SecureServer option can be used to flag to use a secured TSL config when starting the
// server.
//
// The config object can be created by one of the various `SecuredBy...` method on this package
// like `SecuredByX509KeyPair(certFile, keyFile)`.
//
// Important: providing this option erases the settings of the counter-part **InsecureServer** option
// and **PlainTextServer** option, it's mutually exclusive with them.
func SecureServer(config SecureTLSConfig) ServerOption {
	return func(options *serverOptions) {
		options.isPlainText = false
		options.secureTLSConfig = config.asTLSConfig()
	}
}

type SecureTLSConfig interface {
	asTLSConfig() *tls.Config
}

type secureTLSConfigWrapper tls.Config

func (c *secureTLSConfigWrapper) asTLSConfig() *tls.Config {
	return (*tls.Config)(c)
}

// SecuredByX509KeyPair creates a SecureTLSConfig by loading the provided public/private
// X509 pair (`.pem` files format).
func SecuredByX509KeyPair(publicCertFile, privateKeyFile string) (SecureTLSConfig, error) {
	serverCert, err := tls.LoadX509KeyPair(publicCertFile, privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load X509 key pair files: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
	}

	return (*secureTLSConfigWrapper)(config), nil
}

// SecuredByBuiltInSelfSignedCertificate creates a SecureTLSConfig that uses the
// built-in hard-coded certificate found in package `insecure`
// (path `github.com/streamingfast/dgrpc/insecure`).
//
// This certificate is self-signed and distributed publicly over the internet, so
// it's not a safe certificate, can be seen as compromised.
//
// **Never** uses that in a production environment.
func SecuredByBuiltInSelfSignedCertificate() SecureTLSConfig {
	return (*secureTLSConfigWrapper)(&tls.Config{
		Certificates: []tls.Certificate{insecure.Cert},
		ClientCAs:    insecure.CertPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	})
}

// InsecureServer option can be used to flag to use a TSL config using a built-in self-signed certificate
// when starting the server which making it exchange in encrypted format but cannot be considered
// a secure setup.
//
// This is a useful tool for development, **never** use it in a production environment. This is
// equivalent of using the `Secure(SecuredByBuiltInSelfSignedCertificate)` option.
//
// Important: providing this option erases the settings of the counter-part **SecureServer** option
// and **PlainTextServer** option, it's mutually exclusive with them.
func InsecureServer() ServerOption {
	return func(options *serverOptions) {
		options.isPlainText = false
		options.secureTLSConfig = SecuredByBuiltInSelfSignedCertificate().asTLSConfig()
	}
}

// PlainText option can be used to flag to not use a TSL config when starting the
// server which making it exchanges it's data in **plain-text** format (plain binary is
// more accurate here).
//
// Important: providing this option erases the settings of the counter-part **InsecureServer** option
// and **SecureServer** option, it's mutually exclusive with them.
func PlainTextServer() ServerOption {
	return func(options *serverOptions) {
		options.isPlainText = true
		options.secureTLSConfig = nil
	}
}

// WithAuthChecker option can be used to pass a function that will be called
// on connection, validating authentication with 'Authorization: bearer' header
//
// If `enforced` is set to `true`, the token is required and an error is thrown
// when it's not present. If sets to `false`, it's still extracted from the request
// metadata and pass to the auth checker function.
func WithAuthChecker(authChecker AuthCheckerFunc, enforced bool) ServerOption {
	return func(options *serverOptions) {
		options.authCheckerFunc = authChecker
		options.authCheckerEnforced = enforced
	}
}

// HealthCheckOver is a bit field used by the `Server` to decide on what to
// serve the health check when it's defined.
type HealthCheckOver uint8

const (
	// HealthCheckOverHTTP tells the `Server` to serve the health check over an HTTP endpoint (`/healthz` by default)
	HealthCheckOverHTTP HealthCheckOver = 1 << 0

	// HealthCheckOverGRPC tells the `Server` to serve the health check over the `grpc.health.v1.HealthServer` GRPC service.
	HealthCheckOverGRPC HealthCheckOver = 1 << 1
)

func (v HealthCheckOver) isActive(on uint8) bool {
	return (on & uint8(v)) != 0
}

// WithHealthCheck option can be used to automatically register an health check function
// that will be used to determine the health of the server.
//
// If HealthCheckOverHTTP is used, the `Launch` method starts an HTTP
// endpoint '/healthz' to query the `HealthCheck` method provided information.
// The endpoint returns an `OK 200` if `HealthCheck` returned `isReady == true`, an
// `Service Unavailable 503` if `isReady == false`.
//
// The HTTP response body returned depends on the combination of `out` and
// and `err` from `HealthCheck` call:
//
// - Returns `out` as JSON if `out != nil && err == nil`
// - Returns `{"error": err.Error()}` JSON if `out == nil && err != nil`
// - Returns `{"ok": true}` JSON if `out == nil && err == nil`
//
// If HealthCheckOverGRPC is used, the `Launch` method registers within the
// gRPC server a `grpc.health.v1.HealthServer` that uses the `HealthCheck`
// `isReady` field to returning either `HealthCheckResponse_SERVING` or
// `HealthCheckResponse_NOT_SERVING`.
//
// Both option can be provided at a time with `HealthCheckOverHTTP | HealthCheckOverGRPC`
func WithHealthCheck(over HealthCheckOver, check HealthCheck) ServerOption {
	return func(options *serverOptions) {
		options.healthCheck = check
		options.healthCheckOver = over
	}
}

// WithLogger option can be used to pass the logger that should be used to
// log stuff within the various middlewares
func WithLogger(logger *zap.Logger) ServerOption {
	return func(options *serverOptions) {
		options.logger = logger
	}
}

// WithPostUnaryInterceptor option can be used to add your own `grpc.UnaryServerInterceptor`
// after all others defined automatically by the package.
func WithPostUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) ServerOption {
	return func(options *serverOptions) {
		options.postUnaryInterceptors = append(options.postUnaryInterceptors, interceptor)
	}
}

// WithPostStreamInterceptor option can be used to add your own `grpc.StreamServerInterceptor`
// after all others defined automatically by the package.
func WithPostStreamInterceptor(interceptor grpc.StreamServerInterceptor) ServerOption {
	return func(options *serverOptions) {
		options.postStreamInterceptors = append(options.postStreamInterceptors, interceptor)
	}
}

// OverrideTraceID option can be used to force the generation of a new fresh trace ID
// for every gRPC request entering the middleware
func OverrideTraceID() ServerOption {
	return func(options *serverOptions) {
		options.overrideTraceID = true
	}
}

// NewServer creates a new standard fully configured with tracing, logging and
// more.
//
// Deprecated: Use NewGRPCServer version instead, the `NewServer` will return
//             a `dgrpc.Server` instance in an upcoming version.
func NewServer(opts ...ServerOption) *grpc.Server {
	return NewGRPCServer(opts...)
}

// NewGRPCServer creates a new standard fully configured with tracing, logging and
// more.
//
// **Note** Debugging a gRPC server can be done by using `export GODEBUG=http2debug=2`
func NewGRPCServer(opts ...ServerOption) *grpc.Server {
	options := newServerOptions()
	for _, opt := range opts {
		opt(options)
	}

	return newGRPCServer(options)
}

// NewServer creates a new standard fully configured with tracing, logging and
// more.
//
// **Note** Debugging a gRPC server can be done by using `export GODEBUG=http2debug=2`
func newGRPCServer(options *serverOptions) *grpc.Server {
	// GRPC server interceptor that injects in the context the trace_id, if trace_id override option is used it will
	// simply create a new trace_id
	unaryTraceID, streamTraceID := setupTracingInterceptors(options.logger, options.overrideTraceID)

	zopts := []grpc_zap.Option{
		grpc_zap.WithDurationField(grpc_zap.DurationToTimeMillisField),
		grpc_zap.WithDecider(defaultLoggingDecider),
		grpc_zap.WithLevels(defaultServerCodeLevel),
	}

	// Zap base server interceptor
	zapUnaryInterceptor := grpc_zap.UnaryServerInterceptor(options.logger, zopts...)
	zapStreamInterceptor := grpc_zap.StreamServerInterceptor(options.logger.With(zap.String("trace_id", "")), zopts...)

	// Order of interceptors is important here, index order is followed so `{one, two, three}` runs `one` then `two` then `three` passing context along the way
	streamInterceptors := []grpc.StreamServerInterceptor{
		grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
		grpc_prometheus.StreamServerInterceptor,
		zapStreamInterceptor, // zap base server interceptor
		streamTraceID,        // adds trace_id to ctx
	}

	// Order of interceptors is important here, index order is followed so `{one, two, three}` runs `one` then `two` then `three` passing context along the way
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
		grpc_prometheus.UnaryServerInterceptor,
		zapUnaryInterceptor, // zap base server interceptor
		unaryTraceID,        // adds trace_id to ctx
	}

	// Authentication is executed here, so the logger that will run after him can extract information from it
	if options.authCheckerFunc != nil {
		unaryInterceptors = append(unaryInterceptors, unaryAuthChecker(options.authCheckerEnforced, options.authCheckerFunc))
		streamInterceptors = append(streamInterceptors, streamAuthChecker(options.authCheckerEnforced, options.authCheckerFunc))
	}

	// Adds contextualized logger to interceptors, must comes after authenticator since we extract stuff from there is available.
	// The interceptor tries to extract the `trace_id` from the logger and configure the logger to always use it.
	unaryLog, streamLog := setupLoggingInterceptors(options.logger)

	unaryInterceptors = append(unaryInterceptors, unaryLog)
	streamInterceptors = append(streamInterceptors, streamLog)

	// Adds custom defined interceptors, they come after all others
	if len(options.postUnaryInterceptors) > 0 {
		unaryInterceptors = append(unaryInterceptors, options.postUnaryInterceptors...)
	}

	if len(options.postStreamInterceptors) > 0 {
		streamInterceptors = append(streamInterceptors, options.postStreamInterceptors...)
	}

	s := grpc.NewServer(
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
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
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
	if Verbosity <= 2 {
		if code == codes.OK {
			return zap.DebugLevel
		}

		if code == codes.Unauthenticated {
			return zap.DebugLevel
		}

		if code == codes.Unavailable {
			return zap.DebugLevel
		}

		if code == codes.Unknown {
			return zap.DebugLevel
		}
	}

	return grpc_zap.DefaultCodeToLevel(code)
}

// SimpleHealthCheck creates an HTTP handler that server health check response based on `isDown`.
//
// Deprecated: Uses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)`
//             then `go server.Launch()` instead.
func SimpleHealthCheck(isDown func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if isDown() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}
}

// SimpleHTTPServer creates an HTTP server that is able to serve gRPC traffic and has an HTTP handler over HTTP
// if `healthHandler` is specified.
//
// Deprecated: Uses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)`
//             then `go server.Launch()` instead. By default opens a plain-text server, if you require an insecure server
//             like before, use `InsecureServer` option.
func SimpleHTTPServer(srv *grpc.Server, listenAddr string, healthHandler http.HandlerFunc) *http.Server {
	router := mux.NewRouter()

	if healthHandler != nil {
		router.Path("/").HandlerFunc(healthHandler)
		router.Path("/healthz").HandlerFunc(healthHandler)
	}

	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.ServeHTTP(w, r)
	})
	errorLogger, err := zap.NewStdLogAt(zlog, zap.ErrorLevel)
	if err != nil {
		panic(fmt.Errorf("unable to create logger: %w", err))
	}

	addr, insecure := insecureAddr(listenAddr)

	httpSrv := &http.Server{
		Addr:     addr,
		Handler:  router,
		ErrorLog: errorLogger,
	}

	if !insecure {
		httpSrv.TLSConfig = snakeoilTLS()
	}

	return httpSrv
}

// ListenAndServe open a TCP listener and serve gRPC through it the received HTTP server.
//
// Deprecated: Uses `server := dgrpc.NewServer2(options...)` then `go server.Launch()` instead. If you
//             require HTTP health handler, uses option `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)`
//             when configuring your server.
func ListenAndServe(srv *http.Server) error {
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("tcp listen %q: %w", srv.Addr, err)
	}

	if srv.TLSConfig != nil {
		if err := srv.ServeTLS(listener, "", ""); err != nil {
			return fmt.Errorf("tls http server Serve: %w", err)
		}
	} else {
		if err := srv.Serve(listener); err != nil {
			return fmt.Errorf("insecure http server Serve: %w", err)
		}
	}

	return nil
}

func snakeoilTLS() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{insecure.Cert},
		ClientCAs:    insecure.CertPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	}
}

func insecureAddr(in string) (out string, insecure bool) {
	insecure = strings.Contains(in, "*")
	out = strings.Replace(in, "*", "", -1)
	return
}

func unaryAuthChecker(enforced bool, check AuthCheckerFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		childCtx, err := validateAuth(ctx, enforced, check)
		if err != nil {
			return nil, err
		}

		return handler(childCtx, req)
	}
}

func streamAuthChecker(enforced bool, check AuthCheckerFunc) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		childCtx, err := validateAuth(ss.Context(), enforced, check)
		if err != nil {
			return err
		}

		return handler(srv, authenticatedServerStream{ServerStream: ss, authenticatedContext: childCtx})
	}
}

var emptyMetadata = metadata.New(nil)

// validateAuth can auth info to the context
func validateAuth(ctx context.Context, enforced bool, check AuthCheckerFunc) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = emptyMetadata
	}

	token := ""
	authValues := md["authorization"]
	if enforced && len(authValues) <= 0 {
		return ctx, status.Errorf(codes.Unauthenticated, "unable to authenticate request: missing 'authorization' metadata field, you must provide a valid API token through gRPC metadata")
	}

	if len(authValues) > 0 {
		authWords := strings.SplitN(authValues[0], " ", 2)
		if len(authWords) == 1 {
			token = authWords[0]
		} else {
			if strings.ToLower(authWords[0]) != "bearer" {
				return ctx, status.Errorf(codes.Unauthenticated, "unable to authenticate request: invalid value for authorization field")
			}

			token = authWords[1]
		}
	}

	authCtx, err := check(ctx, token, extractGRPCRealIP(ctx, md))
	if err != nil {
		return ctx, status.Errorf(codes.Unauthenticated, "unable to authenticate request: %s", err)
	}

	return authCtx, err
}

var portSuffixRegex = regexp.MustCompile(`:[0-9]{2,5}$`)

func extractGRPCRealIP(ctx context.Context, md metadata.MD) string {
	xForwardedFor := md.Get("x-forwarded-for")
	if len(xForwardedFor) > 0 {
		// When behind a Google Load Balancer, the only two values that we can
		// be sure about are the `n - 2` and `n - 1` (so the last two values
		// in the array). The very last value (`n - 1`) is the Google IP and the
		// `n - 2` value is the actual remote IP that reached the load balancer.
		//
		// When there is more than 2 IPs, all other values prior `n - 2` are
		// those coming from the `X-Forwarded-For` HTTP header received by the load
		// balancer directly, so something a client might have added manually. Since
		// they are coming from an HTTP header and not from Google directly, they
		// can be forged and cannot be trusted.
		//
		// Ideally, to trust the received IP, we should validate it's an actual
		// query coming from Netlify. For now, we are very lenient and trust
		// anything that comes in and looks like an IP.
		//
		// @see https://cloud.google.com/load-balancing/docs/https#x-forwarded-for_header
		if len(xForwardedFor) <= 2 { // 1 or 2
			return strings.TrimSpace(xForwardedFor[0])
		}

		// There is more than 2 addresses, only the element at `n - 2` should be
		// considered, all others cannot be trusted (assuming we got `[a, b, c, d]``,
		// we want to pick element `c` which is at index 2 here so `len(elements) - 2`
		// gives the correct value)
		return strings.TrimSpace(xForwardedFor[len(xForwardedFor)-2]) // more than 2
	}

	if peer, ok := peer.FromContext(ctx); ok {
		switch addr := peer.Addr.(type) {
		case *net.UDPAddr:
			return addr.IP.String()
		case *net.TCPAddr:
			return addr.IP.String()
		default:
			// Hopefully our port removal will work in (almost?) all cases
			return portSuffixRegex.ReplaceAllLiteralString(peer.Addr.String(), "")
		}
	}

	return "0.0.0.0"
}

type authenticatedServerStream struct {
	grpc.ServerStream
	authenticatedContext context.Context
}

func (s authenticatedServerStream) Context() context.Context {
	return s.authenticatedContext
}

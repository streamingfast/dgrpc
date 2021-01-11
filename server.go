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
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dfuse-io/dgrpc/insecure"
	"github.com/gorilla/mux"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
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

type serverOptions struct {
	logger          *zap.Logger
	overrideTraceID bool
	authCheckerFunc AuthCheckerFunc
}

func newServerOptions() *serverOptions {
	return &serverOptions{
		logger:          zlog,
		overrideTraceID: false,
	}
}

// ServerOption represents option that can be used when constructing a gRPC
// server to customize its behavior.
type ServerOption func(*serverOptions)

type AuthCheckerFunc func(ctx context.Context, token, ipAddress string) (context.Context, error)

// WithAuthChecker option can be used to pass a function that will be called
// on connection, validating authentication with 'Authorization: bearer' header
func WithAuthChecker(authChecker AuthCheckerFunc) ServerOption {
	return func(options *serverOptions) {
		options.authCheckerFunc = authChecker
	}
}

// WithLogger option can be used to pass the logger that should be used to
// log stuff within the various middlewares
func WithLogger(logger *zap.Logger) ServerOption {
	return func(options *serverOptions) {
		options.logger = logger
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
// **Note** Debugging a gRPC server can be done by using `export GODEBUG=http2debug=2`
func NewServer(options ...ServerOption) *grpc.Server {
	serverOptions := newServerOptions()
	for _, option := range options {
		option(serverOptions)
	}

	// GRPC server interceptor that injects in the context the trace_id, if trace_id override option is used it will
	// simply create a new trace_id
	unaryTraceID, streamTraceID := setupTracingInterceptors(serverOptions.logger, serverOptions.overrideTraceID)

	// GRPC server intercept that injects in the context the wlooger with the correct trace_id. This is done so that
	// `logging.Logger(ctx,zlog)` always yield a correct fully configured logger.
	unaryLog, streamLog := setupLoggingInterceptors(serverOptions.logger)

	zopts := []grpc_zap.Option{
		grpc_zap.WithDurationField(grpc_zap.DurationToTimeMillisField),
		grpc_zap.WithDecider(defaultLoggingDecider),
		grpc_zap.WithLevels(defaultServerCodeLevel),
	}

	// Zap base server interceptor
	zapUnaryInterceptor := grpc_zap.UnaryServerInterceptor(serverOptions.logger, zopts...)
	zapStreamInterceptor := grpc_zap.StreamServerInterceptor(serverOptions.logger.With(zap.String("trace_id", "")), zopts...)

	streamInterceptors := []grpc.StreamServerInterceptor{
		grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
		grpc_prometheus.StreamServerInterceptor,
		zapStreamInterceptor, // zap base server interceptor
		streamTraceID,        // adds trace_id to ctx
		streamLog,            // adds logger to context
	}

	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
		grpc_prometheus.UnaryServerInterceptor,
		zapUnaryInterceptor, // zap base server interceptor
		unaryTraceID,        // adds trace_id to ctx
		unaryLog,            // adds logger to context
	}

	if serverOptions.authCheckerFunc != nil {
		unaryInterceptors = append(unaryInterceptors, unaryAuthChecker(serverOptions.authCheckerFunc))
		streamInterceptors = append(streamInterceptors, streamAuthChecker(serverOptions.authCheckerFunc))
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

		if code == codes.Unavailable {
			return zap.DebugLevel
		}

		if code == codes.Unknown {
			return zap.DebugLevel
		}
	}

	return grpc_zap.DefaultCodeToLevel(code)
}

func SimpleHealthCheck(isDown func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isDown() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}
}

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

func unaryAuthChecker(check AuthCheckerFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		if err := validateAuth(check, ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func streamAuthChecker(check AuthCheckerFunc) grpc.StreamServerInterceptor {
	return func(srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {

		if err := validateAuth(check, ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// validateAuth can auth info to the context
func validateAuth(check AuthCheckerFunc, ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		err := status.Errorf(codes.Unauthenticated, "unable to authenticate request: missing metadata information, you must provide a valid dfuse API token through gRPC metadata")
		return err
	}

	authValues := md["authorization"]
	if len(authValues) < 1 {
		err := status.Errorf(codes.Unauthenticated, "unable to authenticate request: missing 'authorization' metadata field, you must provide a valid dfuse API token through gRPC metadata")
		return err
	}

	token := strings.TrimPrefix(authValues[0], "Bearer ")
	ip := realIPFromMetadata(md)

	var e error
	ctx, e = check(ctx, token, ip)
	if e != nil {
		return status.Errorf(codes.Unauthenticated, "unable to authenticate request: %s", e)
	}
	return nil
}

func realIPFromMetadata(md metadata.MD) string {
	xff := md.Get("x-forwarded-for")
	forwardIPs := strings.Join(xff, ", ")
	if forwardIPs != "" {
		addresses := strings.Split(forwardIPs, ",")
		if len(addresses) >= 2 {
			return strings.TrimSpace(addresses[len(addresses)-2])
		}
	}

	return "0.0.0.0"
}

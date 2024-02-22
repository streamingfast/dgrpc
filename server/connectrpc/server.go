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

package connectrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"
	gmux "github.com/gorilla/mux"
	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/tracelog"
	"github.com/streamingfast/shutter"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var readyResponse = map[string]interface{}{"is_ready": true}
var notReadyResponse = map[string]interface{}{"is_ready": false}

type errorResponse struct {
	Error error `json:"error"`
}

type ConnectWebServer struct {
	*shutter.Shutter
	logger  *zap.Logger
	options *server.Options

	handler http.Handler
}

type HandlerGetter func(opts ...connect.HandlerOption) (string, http.Handler)

func New(handlerGetters []HandlerGetter, opts ...server.Option) *ConnectWebServer {
	options := server.NewOptions()
	for _, opt := range opts {
		opt(options)
	}

	srv := &ConnectWebServer{
		Shutter: shutter.New(),
		options: options,
		logger:  options.Logger,
	}

	otlInterceptor, err := otelconnect.NewInterceptor()
	if err != nil {
		srv.Shutdown(fmt.Errorf("unable to create otel interceptor: %w", err))
		return nil
	}
	interceptors := append([]connect.Interceptor{
		//connect_go_prometheus.NewInterceptor(), // FIXME this breaks the stream for some reason returning EOF. prometheus disabled
		otlInterceptor,
		tracelog.NewConnectLoggingInterceptor(srv.logger),
		NewErrorsInterceptor(zlog),
	}, options.ConnectExtraInterceptors...)

	if options.ConnectWebStrictContentType {
		interceptors = append(interceptors, ContentTypeInterceptor{allowJSON: options.ConnectWebAllowJSON})
	}

	var connectOpts []connect.HandlerOption
	connectOpts = append(connectOpts, connect.WithInterceptors(interceptors...))

	mux := gmux.NewRouter()

	for _, hg := range handlerGetters {
		path, handler := hg(connectOpts...)
		// Note: connect web handlers return a path prefix, and within the handler
		// they route to the correct GRPC Method
		mux.PathPrefix(path).Handler(handler)
	}

	if len(options.ConnectWebReflectionServices) != 0 {
		reflector := grpcreflect.NewStaticReflector(options.ConnectWebReflectionServices...)
		path, handler := grpcreflect.NewHandlerV1(reflector)
		mux.PathPrefix(path).Handler(handler)
		path, handler = grpcreflect.NewHandlerV1Alpha(reflector)
		mux.PathPrefix(path).Handler(handler)
	}

	if options.HealthCheck != nil {
		mux.Handle("/", http.HandlerFunc(srv.healthCheckHandler))
		mux.Handle("/healthz", http.HandlerFunc(srv.healthCheckHandler))
		mux.Handle(grpchealth.NewHandler(grpchealth.NewStaticChecker()))
	}

	for _, handlerGetter := range options.ConnectWebHTTPHandlers {
		pattern, handler := handlerGetter()
		mux.Handle(pattern, handler)
	}

	var handler http.Handler
	handler = mux
	if options.ConnectWebCORS != nil {
		handler = options.ConnectWebCORS.Handler(mux)
	}

	srv.handler = h2c.NewHandler(handler, &http2.Server{
		MaxConcurrentStreams: 1000,
	})

	return srv
}

// Launch should be run in a go func(), watch for termination by waiting on IsTerminating() channel
func (s *ConnectWebServer) Launch(serverListenerAddress string) {

	s.logger.Info("launching server", zap.String("listen_addr", serverListenerAddress))
	tcpListener, err := net.Listen("tcp", serverListenerAddress)
	if err != nil {
		s.Shutdown(fmt.Errorf("tcp listening to %q: %w", serverListenerAddress, err))
		return
	}

	errorLogger, err := zap.NewStdLogAt(s.logger, zap.ErrorLevel)
	if err != nil {
		s.Shutdown(fmt.Errorf("unable to create logger: %w", err))
		return
	}

	srv := &http.Server{
		Handler:  s.handler,
		ErrorLog: errorLogger,
	}
	s.OnTerminating(func(_ error) { srv.Close() })

	if s.options.SecureTLSConfig != nil {
		s.logger.Info("serving over TLS", zap.String("listen_addr", serverListenerAddress))
		srv.TLSConfig = s.options.SecureTLSConfig
		if err := srv.ServeTLS(tcpListener, "", ""); err != nil {
			s.Shutdown(fmt.Errorf("serve (TLS) failed: %w", err))
			return
		}

	} else if s.options.IsPlainText {
		s.logger.Info("serving plaintext", zap.String("listen_addr", serverListenerAddress))
		if err := srv.Serve(tcpListener); err != nil {
			s.Shutdown(fmt.Errorf("gRPC (over HTTP router) serve failed: %w", err))
			return
		}
	}

	panic("invalid server config, server is not plain-text and no TLS config available, something is wrong, this should never happen")
}

func (s *ConnectWebServer) checkHealth(ctx context.Context) (isReady bool, out interface{}, err error) {
	if s.IsTerminating() {
		return false, nil, nil
	}

	return s.options.HealthCheck(ctx)
}

func (s *ConnectWebServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	isReady, out, err := s.checkHealth(r.Context())

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
}

type ContentTypeInterceptor struct {
	allowJSON bool
}

func (i ContentTypeInterceptor) checkContentType(headers http.Header) error {
	ct := headers.Get("Content-Type")

	switch {
	case strings.HasSuffix(ct, "json"):
		if !i.allowJSON {
			return fmt.Errorf("invalid content-type: 'json' not supported")
		}
	case strings.HasPrefix(ct, "application/connect"), strings.HasPrefix(ct, "application/grpc"):
		return nil
	default:
		zlog.Debug("invalid content-type", zap.String("content_type", ct))
	}
	return fmt.Errorf("invalid content-type %q, only GRPC and Connect are supported", ct)
}

func (i ContentTypeInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.checkContentType(req.Header()); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// Noop
func (i ContentTypeInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i ContentTypeInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.checkContentType(conn.RequestHeader()); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

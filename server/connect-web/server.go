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

package connectweb

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	connect_go "github.com/bufbuild/connect-go"

	otelconnect "github.com/bufbuild/connect-opentelemetry-go"
	//	connect_go_prometheus "github.com/easyCZ/connect-go-prometheus"

	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
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

	handler     http.Handler
	http2Server *http2.Server
}

type HandlerGetter func(opts ...connect_go.HandlerOption) (string, http.Handler)

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

	mux := http.NewServeMux()

	if options.HealthCheck != nil {
		mux.Handle("/healthz", http.HandlerFunc(srv.healthCheckHandler))
	}

	var connectOpts []connect_go.HandlerOption
	connectOpts = append(connectOpts, connect_go.WithInterceptors(
		//		connect_go_prometheus.NewInterceptor(), // FIXME this breaks the stream for some reason, returning EOF
		otelconnect.NewInterceptor(),
		tracelog.NewConnectLoggingInterceptor(srv.logger),
	),
	)

	for _, hg := range handlerGetters {
		pattern, handler := hg(connectOpts...)
		mux.Handle(pattern, handler)
	}

	if len(options.ReflectionServices) != 0 {
		reflector := grpcreflect.NewStaticReflector(options.ReflectionServices...)
		mux.Handle(grpcreflect.NewHandlerV1(reflector))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	}

	var handler http.Handler
	handler = mux
	if options.Cors != nil {
		handler = options.Cors.Handler(mux)
	}

	handler = h2c.NewHandler(handler, &http2.Server{})

	srv.handler = handler

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

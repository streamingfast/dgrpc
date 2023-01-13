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
	"time"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// var balancerDialOption = grpc.WithBalancerName(roundrobin.Name)
var cfg = `
{
  "load_balancing_config": { "round_robin": {} }
}`
var roundrobinDialOption = grpc.WithDefaultServiceConfig(cfg)
var insecureDialOption = grpc.WithInsecure()
var tracingDialOption = grpc.WithStatsHandler(&ocgrpc.ClientHandler{})
var tlsClientDialOption = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))

var largeRecvMsgSizeCallOption = grpc.MaxCallRecvMsgSize(1024 * 1024 * 1024)
var hangOnResolveErrorCallOption = grpc.WaitForReady(true)

// very lightweight keepalives: see https://www.evanjones.ca/grpc-is-tricky.html
var keepaliveDialOption = grpc.WithKeepaliveParams(keepalive.ClientParameters{
	Time:                5 * time.Minute, // send pings every ... when there is no activity
	Timeout:             5 * time.Second, // wait that amount of time for ping ack before considering the connection dead
	PermitWithoutStream: false,
})

// NewInternalClient creates a grpc ClientConn with keep alive, tracing and plain text
// connection (so no TLS involved, the server must also listen to a plain text socket).
// InternalClient also has the default call option to "WaitForReady", which means
// that it will hang indefinitely if the provided remote address does not resolve to
// any valid endpoint. This is a desired behavior for internal services managed by
// "discovery service" mechanisms where the remote endpoint may become ready soon.
//
// It's possible to debug low-level message using `export GODEBUG=http2debug=2`.
func NewInternalClient(remoteAddr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		remoteAddr,
		roundrobinDialOption,
		insecureDialOption,
		keepaliveDialOption,
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		grpc.WithDefaultCallOptions(
			largeRecvMsgSizeCallOption,
			hangOnResolveErrorCallOption,
		),
	)
	return conn, err
}

// NewExternalClient creates a grpc ClientConn with keepalive, tracing and secure TLS
func NewExternalClient(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		roundrobinDialOption,
		keepaliveDialOption,
		tlsClientDialOption,
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		grpc.WithDefaultCallOptions(
			largeRecvMsgSizeCallOption,
		),
	}

	if len(extraOpts) > 0 {
		opts = append(opts, extraOpts...)
	}

	return grpc.Dial(remoteAddr, opts...)
}

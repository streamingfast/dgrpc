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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// var balancerDialOption = grpc.WithBalancerName(roundrobin.Name)
var cfg = `
{
  "load_balancing_config": { "round_robin": {} }
}`
var serviceConfig = grpc.WithDefaultServiceConfig(cfg)

var insecureDialOption = grpc.WithInsecure()
var tracingDialOption = grpc.WithStatsHandler(&ocgrpc.ClientHandler{})
var tlsClientDialOption = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
var maxCallRecvMsgSize = 1024 * 1024 * 1024
var defaultCallOptions = []grpc.CallOption{
	grpc.MaxCallRecvMsgSize(maxCallRecvMsgSize),
	grpc.WaitForReady(true),
}

var keepaliveDialOption = grpc.WithKeepaliveParams(keepalive.ClientParameters{
	Time:                30 * time.Second, // send pings every (x seconds) there is no activity
	Timeout:             10 * time.Second, // wait that amount of time for ping ack before considering the connection dead
	PermitWithoutStream: true,             // send pings even without active streams
})

// NewInternalClient creates a grpc ClientConn with keep alive, tracing and plain text
// connection (so no TLS involved, the server must also listen to a plain text socket).
//
// It's possible to debug low-level message using `export GODEBUG=http2debug=2`.
func NewInternalClient(remoteAddr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		remoteAddr,
		tracingDialOption,
		serviceConfig,
		insecureDialOption,
		keepaliveDialOption,
		grpc.WithDefaultCallOptions(defaultCallOptions...),
	)
	return conn, err
}

// NewExternalClient creates a grpc ClientConn with keepalive, tracing and secure TLS
func NewExternalClient(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		tracingDialOption,
		serviceConfig,
		keepaliveDialOption,
		tlsClientDialOption,
		grpc.WithDefaultCallOptions(defaultCallOptions...),
	}

	if len(extraOpts) > 0 {
		opts = append(opts, extraOpts...)
	}

	return grpc.Dial(remoteAddr, opts...)
}

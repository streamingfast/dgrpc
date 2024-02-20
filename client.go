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
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	xdscreds "google.golang.org/grpc/credentials/xds"
	"google.golang.org/grpc/keepalive"
)

// var balancerDialOption = grpc.WithBalancerName(roundrobin.Name)
var roundrobinDialOption = grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`)
var insecureDialOption = grpc.WithTransportCredentials(insecure.NewCredentials())
var tlsClientDialOption = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))

var largeRecvMsgSizeCallOption = grpc.MaxCallRecvMsgSize(1024 * 1024 * 1024)
var hangOnResolveErrorCallOption = grpc.WaitForReady(true)
var hangOnResolveErrorDialOption = grpc.WithDefaultCallOptions(hangOnResolveErrorCallOption)

// very lightweight keepalives: see https://www.evanjones.ca/grpc-is-tricky.html
var keepaliveDialOption = grpc.WithKeepaliveParams(keepalive.ClientParameters{
	Time:                5 * time.Minute, // send pings every ... when there is no activity
	Timeout:             5 * time.Second, // wait that amount of time for ping ack before considering the connection dead
	PermitWithoutStream: false,
})

// Deprecated: use [NewInternalClientConn] instead
func NewInternalClient(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewInternalClientConn(remoteAddr, extraOpts...)
}

// NewInternalClientConn creates a grpc ClientConn with keep alive, tracing and plain text
// connection (so no TLS involved, the server must also listen to a plain text socket).
// InternalClient also has the default call option to "WaitForReady", which means
// that it will hang indefinitely if the provided remote address does not resolve to
// any valid endpoint. This is a desired behavior for internal services managed by
// "discovery service" mechanisms where the remote endpoint may become ready soon.
//
// It's possible to debug low-level message using `export GODEBUG=http2debug=2`.
func NewInternalClientConn(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewClientConn(remoteAddr, append([]grpc.DialOption{insecureDialOption, hangOnResolveErrorDialOption}, extraOpts...)...)
}

// Deprecated: use [NewInternalNoWaitClientConn] instead
func NewInternalNoWaitClient(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewInternalNoWaitClientConn(remoteAddr, append([]grpc.DialOption{insecureDialOption}, extraOpts...)...)
}

// NewInternalNoWaitClientConn creates a grpc ClientConn with keep alive, tracing and plain text
// connection (so no TLS involved, the server must also listen to a plain text socket).
// InternalClient does not have the default call option to "WaitForReady", which means
// that it will not hang indefinitely if the provided remote address does not resolve to
// any valid endpoint. This is a desired behavior for internal services where the remote endpoint
// is not managed by a "discovery service" mechanism.
func NewInternalNoWaitClientConn(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewClientConn(remoteAddr, append([]grpc.DialOption{insecureDialOption}, extraOpts...)...)
}

// Deprecated: use [NewExternalClientConn] instead
func NewExternalClient(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewExternalClientConn(remoteAddr, extraOpts...)
}

// NewExternalClientConn creates a default gRPC ClientConn with round robin, keepalive, OpenTelemetry tracing,
// large receive message size (max 1 GiB) and TLS secure credentials configured.
func NewExternalClientConn(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewClientConn(remoteAddr, append([]grpc.DialOption{tlsClientDialOption}, extraOpts...)...)
}

// NewClientConn creates a default gRPC ClientConn with round robin, keepalive, OpenTelemetry tracing and
// large receive message size (max 1 GiB) configured.
//
// If the remoteAddr starts with "xds://", it will use xDS credentials, transport credentials are not
// configured and default gRPC applies which is full TLS.
//
// It accepts extra gRPC DialOptions to be passed to the grpc.Dial function.
func NewClientConn(remoteAddr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		roundrobinDialOption,
		keepaliveDialOption,
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		grpc.WithDefaultCallOptions(
			largeRecvMsgSizeCallOption,
		),
	}

	if IsXDSRemoteAddr(remoteAddr) {
		if GetXDSBootstrapFilename() == "" {
			return nil, fmt.Errorf("GRPC_XDS_BOOTSTRAP environment var must be set when using 'xds://' remote addr (%q)", remoteAddr)
		}

		creds, err := xdscreds.NewClientCredentials(xdscreds.ClientOptions{FallbackCreds: insecure.NewCredentials()})
		if err != nil {
			return nil, fmt.Errorf("failed to create xDS credentials: %v", err)
		}

		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	if len(extraOpts) > 0 {
		opts = append(opts, extraOpts...)
	}

	return grpc.Dial(remoteAddr, opts...)
}

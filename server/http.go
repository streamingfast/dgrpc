package server

import (
	"log"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	// DefaultHTTP2ReadIdleTimeout is how long an HTTP/2 connection may stay silent before the
	// server sends a PING frame to check it is still alive. 30s is frequent enough to detect a
	// silently dead load-balancer-to-pod connection well before TCP keepalive would, yet rare
	// enough to be negligible overhead. It matches the (previously ineffective) gRPC keepalive `Time`.
	DefaultHTTP2ReadIdleTimeout = 30 * time.Second

	// DefaultHTTP2PingTimeout is how long the server waits for the PING ack before closing the
	// connection, which frees all its streams. 15s leaves room for a congested but alive peer
	// and bounds the total time to detect a dead peer to ~45s.
	DefaultHTTP2PingTimeout = 15 * time.Second

	// h2cMaxConcurrentStreams is the cleartext (h2c) concurrent stream limit, historical dgrpc value.
	h2cMaxConcurrentStreams = 1000
)

// NewHTTPServer builds the [http.Server] shared by dgrpc servers, with server-side HTTP/2
// PINGs configured from options on both the TLS and the cleartext (h2c) paths.
//
// The returned server wraps handler with [h2c.NewHandler] so cleartext HTTP/2 works, and
// has TLSConfig set from options.SecureTLSConfig (nil when plain-text).
//
// HTTP/2 settings are applied differently per path because `net/http` doesn't share them:
//
//   - TLS: `net/http` negotiates h2 through ALPN and uses its built-in HTTP/2 server, which only
//     reads [http.Server.HTTP2]. An [http2.Server] handed to [h2c.NewHandler] is NOT consulted
//     there. We set [http.HTTP2Config] with the ping settings only, leaving MaxConcurrentStreams
//     unset so it stays at Go's default (250).
//   - h2c: connections are handled by the [http2.Server] given to [h2c.NewHandler], which gets
//     the ping settings and MaxConcurrentStreams of 1000.
func NewHTTPServer(handler http.Handler, options *Options, errorLog *log.Logger) *http.Server {
	srv := &http.Server{
		Handler: h2c.NewHandler(handler, &http2.Server{
			MaxConcurrentStreams: h2cMaxConcurrentStreams,
			ReadIdleTimeout:      options.HTTP2ReadIdleTimeout,
			PingTimeout:          options.HTTP2PingTimeout,
		}),
		ErrorLog: errorLog,
		HTTP2: &http.HTTP2Config{
			SendPingTimeout: options.HTTP2ReadIdleTimeout,
			PingTimeout:     options.HTTP2PingTimeout,
		},
	}

	if options.SecureTLSConfig != nil {
		srv.TLSConfig = options.SecureTLSConfig
	}

	return srv
}

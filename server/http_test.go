package server

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/streamingfast/dgrpc/server/internal/h2test"
)

func startHTTPServer(t *testing.T, options *Options) string {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	srv := NewHTTPServer(handler, options, nil)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	if options.SecureTLSConfig != nil {
		go srv.ServeTLS(listener, "", "")
	} else {
		go srv.Serve(listener)
	}

	return listener.Addr().String()
}

func TestNewHTTPServer(t *testing.T) {
	tests := []struct {
		name                string
		tls                 bool
		expectMaxStreams    uint32
		readIdle, pingAfter time.Duration
	}{
		{"tls keeps go default max streams", true, 250, 200 * time.Millisecond, 3 * time.Second},
		{"h2c keeps 1000 max streams", false, 1000, 200 * time.Millisecond, 3 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := NewOptions()
			WithHTTP2Pings(test.readIdle, time.Second)(options)
			if test.tls {
				WithInsecureServer()(options)
			}

			addr := startHTTPServer(t, options)
			framer, conn := h2test.Dial(t, addr, test.tls)

			probe := h2test.ReadUntilPing(t, framer, conn, test.pingAfter)
			assert.Equal(t, test.expectMaxStreams, probe.Settings[http2.SettingMaxConcurrentStreams])
			assert.GreaterOrEqual(t, probe.PingAfter, test.readIdle/2, "ping should not come before read idle timeout")
		})
	}
}

func TestNewHTTPServer_pingsDisabled(t *testing.T) {
	options := NewOptions()
	WithHTTP2Pings(0, 0)(options)

	addr := startHTTPServer(t, options)
	framer, conn := h2test.Dial(t, addr, false)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return // read deadline, no ping seen
		}
		if f, ok := frame.(*http2.PingFrame); ok && !f.IsAck() {
			require.Fail(t, "unexpected PING with pings disabled")
		}
	}
}

func TestNewOptions_http2PingDefaults(t *testing.T) {
	options := NewOptions()
	assert.Equal(t, 30*time.Second, options.HTTP2ReadIdleTimeout)
	assert.Equal(t, 15*time.Second, options.HTTP2PingTimeout)
}

package standard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/http2"

	"github.com/streamingfast/dgrpc/server"
	"github.com/streamingfast/dgrpc/server/internal/h2test"
)

func TestServer_http2(t *testing.T) {
	tests := []struct {
		name             string
		tls              bool
		expectMaxStreams uint32
	}{
		{"tls", true, 250},
		{"h2c", false, 1000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Health over HTTP so the server goes through the TLS-capable http.Server path
			opts := []server.Option{
				server.WithHealthCheck(server.HealthCheckOverHTTP, func(ctx context.Context) (bool, any, error) { return true, nil, nil }),
				server.WithHTTP2Pings(200*time.Millisecond, time.Second),
			}
			if test.tls {
				opts = append(opts, server.WithInsecureServer())
			}

			options := server.NewOptions()
			for _, opt := range opts {
				opt(options)
			}

			srv := NewServer(options)
			addr := h2test.FreeAddr(t)
			go srv.Launch(addr)
			t.Cleanup(func() { srv.httpServer.Close() })

			framer, conn := h2test.Dial(t, addr, test.tls)
			probe := h2test.ReadUntilPing(t, framer, conn, 3*time.Second)
			assert.Equal(t, test.expectMaxStreams, probe.Settings[http2.SettingMaxConcurrentStreams])
		})
	}
}

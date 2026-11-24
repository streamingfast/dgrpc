// Package h2test holds raw HTTP/2 probing helpers shared by the server tests.
package h2test

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// FreeAddr returns a localhost address that was free a moment ago.
func FreeAddr(t testing.TB) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	return l.Addr().String()
}

// Dial connects to addr and returns a framer on it after having exchanged the connection
// preface, either over TLS (negotiating `h2` through ALPN) or in cleartext (h2c prior knowledge).
// It retries for a few seconds so it can be called right after launching a server in a goroutine.
func Dial(t testing.TB, addr string, useTLS bool) (*http2.Framer, net.Conn) {
	t.Helper()

	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	for {
		var err error
		if useTLS {
			conn, err = tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2"}})
		} else {
			conn, err = net.Dial("tcp", addr)
		}
		if err == nil {
			break
		}

		require.True(t, time.Now().Before(deadline), "dialing %s: %s", addr, err)
		time.Sleep(25 * time.Millisecond)
	}

	if tlsConn, ok := conn.(*tls.Conn); ok {
		require.Equal(t, "h2", tlsConn.ConnectionState().NegotiatedProtocol, "server did not negotiate h2 through ALPN")
	}
	t.Cleanup(func() { conn.Close() })

	_, err := conn.Write([]byte(http2.ClientPreface))
	require.NoError(t, err)

	framer := http2.NewFramer(conn, conn)
	require.NoError(t, framer.WriteSettings())

	return framer, conn
}

// Probe holds what was observed on a connection.
type Probe struct {
	Settings map[http2.SettingID]uint32
	// PingAfter is the time elapsed between the server SETTINGS and the first (non-ack) PING.
	PingAfter time.Duration
}

// ReadUntilPing reads frames until the server sends a PING (acking server SETTINGS meanwhile)
// and fails if none comes within timeout.
func ReadUntilPing(t testing.TB, framer *http2.Framer, conn net.Conn, timeout time.Duration) Probe {
	t.Helper()

	probe := Probe{Settings: map[http2.SettingID]uint32{}}
	start := time.Now()
	deadline := start.Add(timeout)

	for {
		require.NoError(t, conn.SetReadDeadline(deadline))
		frame, err := framer.ReadFrame()
		require.NoError(t, err, "no PING received from server within %s", timeout)

		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if f.IsAck() {
				continue
			}
			_ = f.ForeachSetting(func(s http2.Setting) error {
				probe.Settings[s.ID] = s.Val
				return nil
			})
			require.NoError(t, framer.WriteSettingsAck())
		case *http2.PingFrame:
			if f.IsAck() {
				continue
			}
			probe.PingAfter = time.Since(start)
			require.NoError(t, framer.WritePing(true, f.Data))
			return probe
		}
	}
}

// ReadSettings reads frames until the server's initial non-ack SETTINGS and returns its values.
func ReadSettings(t testing.TB, framer *http2.Framer, conn net.Conn) map[http2.SettingID]uint32 {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		frame, err := framer.ReadFrame()
		require.NoError(t, err)
		if f, ok := frame.(*http2.SettingsFrame); ok && !f.IsAck() {
			out := map[http2.SettingID]uint32{}
			_ = f.ForeachSetting(func(s http2.Setting) error {
				out[s.ID] = s.Val
				return nil
			})
			require.NoError(t, framer.WriteSettingsAck())
			return out
		}
	}
}

package standard

import (
	"testing"

	"github.com/streamingfast/dgrpc/server"

	"github.com/stretchr/testify/assert"
)

func TestHealthOver_isActive(t *testing.T) {
	tests := []struct {
		name     string
		on       uint8
		target   server.HealthCheckOver
		expected bool
	}{
		{"on http|grpc, testing grpc", uint8(server.HealthCheckOverHTTP | server.HealthCheckOverGRPC), server.HealthCheckOverGRPC, true},
		{"on http|grpc, testing http", uint8(server.HealthCheckOverGRPC | server.HealthCheckOverHTTP), server.HealthCheckOverHTTP, true},

		{"on http, testing grpc", uint8(server.HealthCheckOverHTTP), server.HealthCheckOverGRPC, false},
		{"on http, testing http", uint8(server.HealthCheckOverHTTP), server.HealthCheckOverHTTP, true},
		{"on grpc, testing grpc", uint8(server.HealthCheckOverGRPC), server.HealthCheckOverGRPC, true},
		{"on grpc, testing http", uint8(server.HealthCheckOverGRPC), server.HealthCheckOverHTTP, false},

		{"empty, testing grpc", uint8(0), server.HealthCheckOverGRPC, false},
		{"empty, testing http", uint8(0), server.HealthCheckOverHTTP, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.target.IsActive(test.on))
		})
	}
}

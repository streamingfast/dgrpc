package dgrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthOver_isActive(t *testing.T) {
	tests := []struct {
		name     string
		on       uint8
		target   HealthCheckOver
		expected bool
	}{
		{"on http|grpc, testing grpc", uint8(HealthCheckOverHTTP | HealthCheckOverGRPC), HealthCheckOverGRPC, true},
		{"on http|grpc, testing http", uint8(HealthCheckOverGRPC | HealthCheckOverHTTP), HealthCheckOverHTTP, true},

		{"on http, testing grpc", uint8(HealthCheckOverHTTP), HealthCheckOverGRPC, false},
		{"on http, testing http", uint8(HealthCheckOverHTTP), HealthCheckOverHTTP, true},
		{"on grpc, testing grpc", uint8(HealthCheckOverGRPC), HealthCheckOverGRPC, true},
		{"on grpc, testing http", uint8(HealthCheckOverGRPC), HealthCheckOverHTTP, false},

		{"empty, testing grpc", uint8(0), HealthCheckOverGRPC, false},
		{"empty, testing http", uint8(0), HealthCheckOverHTTP, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.target.isActive(test.on))
		})
	}
}

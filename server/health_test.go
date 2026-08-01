package server

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pbhealth "google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthGRPCHandler_List(t *testing.T) {
	checkError := errors.New("check failed")

	tests := []struct {
		name     string
		check    HealthCheck
		expected map[string]*pbhealth.HealthCheckResponse
		wantErr  error
	}{
		{
			name:  "ready reports serving under the empty service name",
			check: func(ctx context.Context) (bool, any, error) { return true, nil, nil },
			expected: map[string]*pbhealth.HealthCheckResponse{
				"": {Status: pbhealth.HealthCheckResponse_SERVING},
			},
		},
		{
			name:  "not ready reports not serving",
			check: func(ctx context.Context) (bool, any, error) { return false, nil, nil },
			expected: map[string]*pbhealth.HealthCheckResponse{
				"": {Status: pbhealth.HealthCheckResponse_NOT_SERVING},
			},
		},
		{
			name:    "check error is propagated",
			check:   func(ctx context.Context) (bool, any, error) { return false, nil, checkError },
			wantErr: checkError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := NewHealthGRPCHandler(test.check).List(context.Background(), &pbhealth.HealthListRequest{})
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, response.GetStatuses(), len(test.expected))
			for service, expected := range test.expected {
				actual, found := response.GetStatuses()[service]
				require.True(t, found, "no status for service %q", service)
				assert.Equal(t, expected.GetStatus(), actual.GetStatus())
			}
		})
	}
}

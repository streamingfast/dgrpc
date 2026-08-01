package connectrpc

import (
	"context"
	"errors"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/streamingfast/dgrpc/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthGRPCHandler_List(t *testing.T) {
	checkError := errors.New("check failed")

	tests := []struct {
		name     string
		check    server.HealthCheck
		expected grpc_health_v1.HealthCheckResponse_ServingStatus
		wantErr  error
	}{
		{
			name:     "ready reports serving under the empty service name",
			check:    func(ctx context.Context) (bool, any, error) { return true, nil, nil },
			expected: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:     "not ready reports not serving",
			check:    func(ctx context.Context) (bool, any, error) { return false, nil, nil },
			expected: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:    "check error is propagated",
			check:   func(ctx context.Context) (bool, any, error) { return false, nil, checkError },
			wantErr: checkError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHealthGRPCHandler(test.check)
			response, err := handler.List(context.Background(), connect.NewRequest(&grpc_health_v1.HealthListRequest{}))
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			statuses := response.Msg.GetStatuses()
			require.Len(t, statuses, 1)

			actual, found := statuses[""]
			require.True(t, found, `no status for the empty service name`)
			assert.Equal(t, test.expected, actual.GetStatus())
		})
	}
}

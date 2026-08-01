package connectrpc

import (
	"context"
	"time"

	connect "connectrpc.com/connect"
	"github.com/streamingfast/dgrpc/server"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type HealthGRPCHandler struct {
	check server.HealthCheck
}

func NewHealthGRPCHandler(check server.HealthCheck) *HealthGRPCHandler {
	return &HealthGRPCHandler{check: check}
}

func (c HealthGRPCHandler) Check(ctx context.Context, req *connect.Request[grpc_health_v1.HealthCheckRequest]) (*connect.Response[grpc_health_v1.HealthCheckResponse], error) {
	status, err := c.healthStatus(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&grpc_health_v1.HealthCheckResponse{Status: status}), nil
}

// List returns a snapshot of the health of every service served by this handler. The
// handler serves a single, server-wide health check, so the snapshot always contains a
// single entry keyed by the empty service name.
func (c HealthGRPCHandler) List(ctx context.Context, req *connect.Request[grpc_health_v1.HealthListRequest]) (*connect.Response[grpc_health_v1.HealthListResponse], error) {
	status, err := c.healthStatus(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&grpc_health_v1.HealthListResponse{
		Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
			"": {Status: status},
		},
	}), nil
}

func (c HealthGRPCHandler) Watch(ctx context.Context, req *connect.Request[grpc_health_v1.HealthCheckRequest], stream *connect.ServerStream[grpc_health_v1.HealthCheckResponse]) error {
	currentStatus := grpc_health_v1.HealthCheckResponse_UNKNOWN

	waitTime := 0 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(waitTime):
			newStatus, _ := c.healthStatus(ctx)
			waitTime = 5 * time.Second

			if newStatus != currentStatus {
				currentStatus = newStatus

				resp := &grpc_health_v1.HealthCheckResponse{Status: currentStatus}
				if err := stream.Send(resp); err != nil {
					return err
				}
			}
		}
	}
}

func (c HealthGRPCHandler) healthStatus(ctx context.Context) (grpc_health_v1.HealthCheckResponse_ServingStatus, error) {
	isReady, _, err := c.check(ctx)
	if err != nil {
		return grpc_health_v1.HealthCheckResponse_SERVICE_UNKNOWN, err
	}

	if isReady {
		return grpc_health_v1.HealthCheckResponse_SERVING, nil
	}

	return grpc_health_v1.HealthCheckResponse_NOT_SERVING, nil
}

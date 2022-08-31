package server

import (
	"context"
	"time"

	pbhealth "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	// HealthCheckOverHTTP tells the `StandardServer` to serve the health check over an HTTP endpoint (`/healthz` by default)
	HealthCheckOverHTTP HealthCheckOver = 1 << 0

	// HealthCheckOverGRPC tells the `StandardServer` to serve the health check over the `grpc.health.v1.HealthServer` GRPC service.
	HealthCheckOverGRPC HealthCheckOver = 1 << 1
)

type HealthCheck func(ctx context.Context) (isReady bool, out interface{}, err error)

// HealthCheckOver is a bit field used by the `StandardServer` to decide on what to
// serve the health check when it's defined.
type HealthCheckOver uint8

func (v HealthCheckOver) IsActive(on uint8) bool {
	return (on & uint8(v)) != 0
}

type HealthGRPCHandler struct {
	check HealthCheck
}

func NewHealthGRPCHandler(check HealthCheck) *HealthGRPCHandler {
	return &HealthGRPCHandler{check: check}
}

func (c HealthGRPCHandler) Check(ctx context.Context, _ *pbhealth.HealthCheckRequest) (*pbhealth.HealthCheckResponse, error) {
	status, err := c.healthStatus(ctx)
	if err != nil {
		return nil, err
	}

	return &pbhealth.HealthCheckResponse{Status: status}, nil
}

func (c HealthGRPCHandler) Watch(req *pbhealth.HealthCheckRequest, stream pbhealth.Health_WatchServer) error {
	currentStatus := pbhealth.HealthCheckResponse_SERVICE_UNKNOWN
	waitTime := 0 * time.Second

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(waitTime):
			newStatus, _ := c.healthStatus(stream.Context())
			waitTime = 5 * time.Second

			if newStatus != currentStatus {
				currentStatus = newStatus

				if err := stream.Send(&pbhealth.HealthCheckResponse{Status: currentStatus}); err != nil {
					return err
				}
			}
		}
	}
}

func (c HealthGRPCHandler) healthStatus(ctx context.Context) (pbhealth.HealthCheckResponse_ServingStatus, error) {
	isReady, _, err := c.check(ctx)
	if err != nil {
		return pbhealth.HealthCheckResponse_SERVICE_UNKNOWN, err
	}

	if isReady {
		return pbhealth.HealthCheckResponse_SERVING, nil
	}

	return pbhealth.HealthCheckResponse_NOT_SERVING, nil
}

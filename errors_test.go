package dgrpc

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAsGRPCError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want *status.Status
	}{
		{"nil", nil, nil},
		{"not grpc status, direct level", errors.New("level 1"), nil},
		{"not grpc status, second level", fmt.Errorf("level 1: %w", errors.New("level 2")), nil},
		{"grpc status, direct level", status.Error(codes.Aborted, "err"), status.New(codes.Aborted, "err")},
		{"grpc status, second level", fmt.Errorf("level 1: %w", status.Error(codes.Aborted, "err")), status.New(codes.Aborted, "err")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AsGRPCError(tt.err))
		})
	}
}

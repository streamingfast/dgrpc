package dgrpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsGRPCErrorCode is a convenience to reduce code when using [AsGRPCError]:
//
//	if err := AsGRPCError(err); err != nil && err.Code() == code {
//		return true
//	}
//
//	return false
func IsGRPCErrorCode(err error, code codes.Code) bool {
	if err := AsGRPCError(err); err != nil && err.Code() == code {
		return true
	}

	return false
}

// AsGRPCError recursively finds the first value [Status] representation out of
// this error stack. Refers to [status.FromError] for details how this is tested.
//
// If no such [Status] can be found, nil is returned, expected usage is:
//
//	if err := AsGRPCError(err); err != nil && err.Code == codes.Canceled {
//		// Do something
//	}
func AsGRPCError(err error) *status.Status {
	for next := err; next != nil; next = errors.Unwrap(next) {
		if status, ok := status.FromError(next); ok {
			return status
		}
	}

	return nil
}

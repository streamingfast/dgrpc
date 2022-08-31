package dgrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

type AuthCheckerFunc func(ctx context.Context, token, ipAddress string) (context.Context, error)

func UnaryAuthChecker(enforced bool, check AuthCheckerFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		childCtx, err := validateAuth(ctx, enforced, check)
		if err != nil {
			return nil, err
		}

		return handler(childCtx, req)
	}
}

func StreamAuthChecker(enforced bool, check AuthCheckerFunc) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		childCtx, err := validateAuth(ss.Context(), enforced, check)
		if err != nil {
			return err
		}

		return handler(srv, authenticatedServerStream{ServerStream: ss, authenticatedContext: childCtx})
	}
}

// validateAuth can auth info to the context
func validateAuth(ctx context.Context, enforced bool, check AuthCheckerFunc) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = emptyMetadata
	}

	token := ""
	authValues := md["authorization"]
	if enforced && len(authValues) <= 0 {
		return ctx, status.Errorf(codes.Unauthenticated, "unable to authenticate request: missing 'authorization' metadata field, you must provide a valid API token through gRPC metadata")
	}

	if len(authValues) > 0 {
		authWords := strings.SplitN(authValues[0], " ", 2)
		if len(authWords) == 1 {
			token = authWords[0]
		} else {
			if strings.ToLower(authWords[0]) != "bearer" {
				return ctx, status.Errorf(codes.Unauthenticated, "unable to authenticate request: invalid value for authorization field")
			}

			token = authWords[1]
		}
	}

	authCtx, err := check(ctx, token, extractGRPCRealIP(ctx, md))
	if err != nil {
		return ctx, status.Errorf(codes.Unauthenticated, "unable to authenticate request: %s", err)
	}

	return authCtx, err
}

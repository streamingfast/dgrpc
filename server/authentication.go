package server

import (
	"context"
	"net"
	"regexp"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var portSuffixRegex = regexp.MustCompile(`:[0-9]{2,5}$`)
var EmptyMetadata = metadata.New(nil)

type AuthenticatedServerStream struct {
	grpc.ServerStream
	AuthenticatedContext context.Context
}

func (s AuthenticatedServerStream) Context() context.Context {
	return s.AuthenticatedContext
}

func extractGRPCRealIP(ctx context.Context, md metadata.MD) string {
	xForwardedFor := md.Get("x-forwarded-for")
	if len(xForwardedFor) > 0 {
		// When behind a Google Load Balancer, the only two values that we can
		// be sure about are the `n - 2` and `n - 1` (so the last two values
		// in the array). The very last value (`n - 1`) is the Google IP and the
		// `n - 2` value is the actual remote IP that reached the load balancer.
		//
		// When there is more than 2 IPs, all other values prior `n - 2` are
		// those coming from the `X-Forwarded-For` HTTP header received by the load
		// balancer directly, so something a client might have added manually. Since
		// they are coming from an HTTP header and not from Google directly, they
		// can be forged and cannot be trusted.
		//
		// Ideally, to trust the received IP, we should validate it's an actual
		// query coming from Netlify. For now, we are very lenient and trust
		// anything that comes in and looks like an IP.
		//
		// @see https://cloud.google.com/load-balancing/docs/https#x-forwarded-for_header
		if len(xForwardedFor) <= 2 { // 1 or 2
			return strings.TrimSpace(xForwardedFor[0])
		}

		// There is more than 2 addresses, only the element at `n - 2` should be
		// considered, all others cannot be trusted (assuming we got `[a, b, c, d]``,
		// we want to pick element `c` which is at index 2 here so `len(elements) - 2`
		// gives the correct value)
		return strings.TrimSpace(xForwardedFor[len(xForwardedFor)-2]) // more than 2
	}

	if peer, ok := peer.FromContext(ctx); ok {
		switch addr := peer.Addr.(type) {
		case *net.UDPAddr:
			return addr.IP.String()
		case *net.TCPAddr:
			return addr.IP.String()
		default:
			// Hopefully our port removal will work in (almost?) all cases
			return portSuffixRegex.ReplaceAllLiteralString(peer.Addr.String(), "")
		}
	}

	return "0.0.0.0"
}

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

		return handler(srv, AuthenticatedServerStream{ServerStream: ss, AuthenticatedContext: childCtx})
	}
}

// validateAuth can auth info to the context
func validateAuth(ctx context.Context, enforced bool, check AuthCheckerFunc) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = EmptyMetadata
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

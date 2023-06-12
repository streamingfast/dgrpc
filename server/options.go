package server

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/cors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Options struct {
	HealthCheck     HealthCheck
	HealthCheckOver HealthCheckOver
	Logger          *zap.Logger
	IsPlainText     bool
	OverrideTraceID bool

	// GRPC-only options
	ServerOptions          []grpc.ServerOption
	PostUnaryInterceptors  []grpc.UnaryServerInterceptor
	PostStreamInterceptors []grpc.StreamServerInterceptor
	Registrator            func(gs *grpc.Server)
	SecureTLSConfig        *tls.Config

	// ConnectWeb-only options
	ReflectionServices []string
	Cors               *cors.Cors

	// discovery-service-only options
	ServiceDiscoveryURL *url.URL
}

func NewOptions() *Options {
	return &Options{
		Logger:          zlog,
		IsPlainText:     true,
		OverrideTraceID: false,
	}
}

// Option represents option that can be used when constructing a gRPC
// server to customize its behavior.
type Option func(*Options)

func WithServiceDiscoveryURL(u *url.URL) Option {
	return func(options *Options) {
		options.ServiceDiscoveryURL = u
	}
}

// WithSecureServer option can be used to flag to use a secured TSL config when starting the
// server.
//
// The config object can be created by one of the various `SecuredBy...` method on this package
// like `SecuredByX509KeyPair(certFile, keyFile)`.
//
// Important: providing this option erases the settings of the counter-part **InsecureServer** option
// and **PlainTextServer** option, it's mutually exclusive with them.
func WithSecureServer(config SecureTLSConfig) Option {
	return func(options *Options) {
		options.IsPlainText = false
		options.SecureTLSConfig = config.asTLSConfig()
	}
}

// WithCORS Will apply the CORS policy to your server
// Only works with connectweb servers
func WithCORS(c *cors.Cors) Option {
	return func(options *Options) {
		options.Cors = c
	}
}

// WithPermissiveCORS is for development environments, as it disables any CORS validation
// Only works with connectweb servers
func WithPermissiveCORS() Option {
	return func(options *Options) {
		options.Cors = cors.New(cors.Options{
			AllowedMethods: []string{
				http.MethodHead,
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
			},
			AllowOriginFunc: func(origin string) bool {
				// Allow all origins, which effectively disables CORS.
				return true
			},
			AllowedHeaders: []string{"*"},
			ExposedHeaders: []string{
				// Content-Type is in the default safelist.
				"Accept",
				"Accept-Encoding",
				"Accept-Post",
				"Connect-Accept-Encoding",
				"Connect-Content-Encoding",
				"Content-Encoding",
				"Grpc-Accept-Encoding",
				"Grpc-Encoding",
				"Grpc-Message",
				"Grpc-Status",
				"Grpc-Status-Details-Bin",
			},
			MaxAge: int(2 * time.Hour / time.Second),
		})

	}
}

// WithReflection enables GRPC reflection for the given location string. It be called multiple times
// Only works with connectweb servers
func WithReflection(location string) Option {
	return func(options *Options) {
		options.ReflectionServices = append(options.ReflectionServices, location)
	}
}

// WithInsecureServer option can be used to flag to use a TSL config using a built-in self-signed certificate
// when starting the server which making it exchange in encrypted format but cannot be considered
// a secure setup.
//
// This is a useful tool for development, **never** use it in a production environment. This is
// equivalent of using the `Secure(SecuredByBuiltInSelfSignedCertificate)` option.
//
// Important: providing this option erases the settings of the counter-part **SecureServer** option
// and **PlainTextServer** option, it's mutually exclusive with them.
func WithInsecureServer() Option {
	return func(options *Options) {
		options.IsPlainText = false
		options.SecureTLSConfig = SecuredByBuiltInSelfSignedCertificate().asTLSConfig()
	}
}

// WithPlainTextServer option can be used to flag to not use a TSL config when starting the
// server which making it exchanges it's data in **plain-text** format (plain binary is
// more accurate here).
//
// Important: providing this option erases the settings of the counter-part **InsecureServer** option
// and **SecureServer** option, it's mutually exclusive with them.
func WithPlainTextServer() Option {
	return func(options *Options) {
		options.IsPlainText = true
		options.SecureTLSConfig = nil
	}
}

// WithHealthCheck option can be used to automatically register an health check function
// that will be used to determine the health of the server.
//
// If HealthCheckOverHTTP is used, the `Launch` method starts an HTTP
// endpoint '/healthz' to query the `HealthCheck` method provided information.
// The endpoint returns an `OK 200` if `HealthCheck` returned `isReady == true`, an
// `Service Unavailable 503` if `isReady == false`.
//
// The HTTP response body returned depends on the combination of `out` and
// and `err` from `HealthCheck` call:
//
// - Returns `out` as JSON if `out != nil && err == nil`
// - Returns `{"error": err.Error()}` JSON if `out == nil && err != nil`
// - Returns `{"ok": true}` JSON if `out == nil && err == nil`
//
// If HealthCheckOverGRPC is used, the `Launch` method registers within the
// gRPC server a `grpc.health.v1.HealthServer` that uses the `HealthCheck`
// `isReady` field to returning either `HealthCheckResponse_SERVING` or
// `HealthCheckResponse_NOT_SERVING`.
//
// Both option can be provided at a time with `HealthCheckOverHTTP | HealthCheckOverGRPC`
func WithHealthCheck(over HealthCheckOver, check HealthCheck) Option {
	return func(options *Options) {
		options.HealthCheck = check
		options.HealthCheckOver = over
	}
}

// WithLogger option can be used to pass the logger that should be used to
// log stuff within the various middlewares
func WithLogger(logger *zap.Logger) Option {
	return func(options *Options) {
		options.Logger = logger
	}
}

// WithRegisterService option can be used to register the different gRPC
// services that this server is going to support.
func WithRegisterService(registrator func(gs *grpc.Server)) Option {
	return func(options *Options) {
		options.Registrator = registrator
	}
}

// WithPostUnaryInterceptor option can be used to add your own `grpc.UnaryServerInterceptor`
// after all others defined automatically by the package.
func WithPostUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return func(options *Options) {
		options.PostUnaryInterceptors = append(options.PostUnaryInterceptors, interceptor)
	}
}

// WithPostStreamInterceptor option can be used to add your own `grpc.StreamServerInterceptor`
// after all others defined automatically by the package.
func WithPostStreamInterceptor(interceptor grpc.StreamServerInterceptor) Option {
	return func(options *Options) {
		options.PostStreamInterceptors = append(options.PostStreamInterceptors, interceptor)
	}
}

// WithGRPCServerOptions let you configure (or re-configure) the set of
// gRPC option passed to [grpc.NewServer] call. The options are appended
// at the end of gRPC server options computed by `dgrpc`. Using WithGRPCServerOptions
// you can for example control the [grpc.MaxRecvMsgSize], [grpc.MaxSendMsgSize],
// [grpc.KeepaliveEnforcementPolicy], [grpc.KeepaliveParams] and more.
//
// It's important to note that if you pass [grpc_middleware.WithStreamServerChain] or
// [grpc_middleware.WithUnaryServerChain], you are going to override [WithPostUnaryInterceptor]
// and [WithPostUnaryInterceptor] since those are configured via [grpc.NewServer].
func WithGRPCServerOptions(opts ...grpc.ServerOption) Option {
	return func(options *Options) {
		options.ServerOptions = opts
	}
}

// OverrideTraceID option can be used to force the generation of a new fresh trace ID
// for every gRPC request entering the middleware
func OverrideTraceID() Option {
	return func(options *Options) {
		options.OverrideTraceID = true
	}
}

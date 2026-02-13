package server

import (
	"time"

	"google.golang.org/grpc"
)

type Server interface {
	// RegisterService registers one or more gRPC service to the server. The service must be
	// registered **before** the `Launch()` method has been called otherwise this is a no-op.
	//
	// This is a convenience method that calls `f(ServiceRegistrar())`.
	RegisterService(f func(gs grpc.ServiceRegistrar))

	// ServiceRegistrar returns the `grpc.ServiceRegistrar` that can be used to register
	// more services to the server. The returned `grpc.ServiceRegistrar` is only effective
	// **before** the `Launch()` method has been called otherwise this is a no-op.
	ServiceRegistrar() grpc.ServiceRegistrar

	// Launch the server and listen on the given address, this is a blocking call and
	// will block the current goroutine until the server is terminated by a call to
	// Shutdown() or an error occurs (e.g. the server fails to start).
	Launch(serverListenerAddress string)

	OnTerminated(f func(err error))

	// Shutdown the server, this is a blocking call and will block the current goroutine
	// until the server is fully shutdown. The shutdown is performed gracefully, meaning
	// that the server will wait for all active connections to be closed before terminating.
	//
	// If not all connection are closed within the given timeout, the server will force
	// shutdown.
	//
	// Best practices for best functionality is to trap the SIGINT/SIGTERM signals and
	// set the process as unready right away and then initiate the shutdown once the
	// delay has passed. By doing this, you are going to give the load balancer some
	// time to stop sending traffic to this server before it is terminated.
	Shutdown(timeout time.Duration)
}

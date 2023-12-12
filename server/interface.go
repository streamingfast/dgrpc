package server

import (
	"time"

	"google.golang.org/grpc"
)

type Server interface {
	RegisterService(f func(gs grpc.ServiceRegistrar))

	// Launch the server and listen on the given address, this is a blocking call and
	// will block the current goroutine until the server is terminated by a call to
	// Shutdown() or an error occurs (e.g. the server fails to start).
	Launch(serverListenerAddress string)

	ServiceRegistrar() grpc.ServiceRegistrar

	OnTerminated(f func(err error))
	Shutdown(timeout time.Duration)
}

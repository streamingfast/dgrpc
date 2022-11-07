package server

import (
	"time"

	"google.golang.org/grpc"
)

type Server interface {
	RegisterService(f func(gs grpc.ServiceRegistrar))
	Launch(serverListenerAddress string)

	ServiceRegistrar() grpc.ServiceRegistrar

	OnTerminated(f func(err error))
	Shutdown(timeout time.Duration)
}

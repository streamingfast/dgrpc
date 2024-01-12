package dgrpc

import (
	"context"
	"fmt"

	"go.uber.org/atomic"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
)

type RoundRobinConnPool struct {
	conns []*grpc.ClientConn
	idx   *atomic.Uint32
}

func NewRoundRobinConnPool(conns []*grpc.ClientConn) *RoundRobinConnPool {
	return &RoundRobinConnPool{
		conns: conns,
		idx:   atomic.NewUint32(0),
	}
}

func (p *RoundRobinConnPool) Num() int {
	return len(p.conns)
}

func (p *RoundRobinConnPool) Conn() *grpc.ClientConn {
	newIdx := p.idx.Inc()
	return p.conns[newIdx%uint32(len(p.conns))]
}

func (p *RoundRobinConnPool) Close() error {
	var errs error
	for _, conn := range p.conns {
		if err := conn.Close(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if errs != nil {
		return fmt.Errorf("failed to close all connections: %w", errs)
	}

	return errs
}

func (p *RoundRobinConnPool) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return p.Conn().Invoke(ctx, method, args, reply, opts...)
}

func (p *RoundRobinConnPool) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return p.Conn().NewStream(ctx, desc, method, opts...)
}

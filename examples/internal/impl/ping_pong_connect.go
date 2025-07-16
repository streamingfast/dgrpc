package impl

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	pbacme "github.com/streamingfast/dgrpc/examples/internal/pb/acme/v1"
	"github.com/streamingfast/dgrpc/examples/internal/pb/acme/v1/pbacmeconnect"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// Ensure we implement the Connect handler interface
var _ pbacmeconnect.PingPongServiceHandler = (*pingPongConnectServer)(nil)

type pingPongConnectServer struct {
	ID            string
	Logger        *zap.Logger
	activeRequest *atomic.Uint64
}

func NewPingPongConnectServer(id string, logger *zap.Logger) pbacmeconnect.PingPongServiceHandler {
	s := &pingPongConnectServer{
		ID:            id,
		Logger:        logger,
		activeRequest: atomic.NewUint64(0),
	}

	// In a real world scenario, you would tie this to a proper metrics system
	// and tie it to a proper shutdown mechanism so that the goroutine and timer are stopped
	// properly
	go func() {
		for range time.Tick(5 * time.Second) {
			logger.Info("Active Connect requests", zap.Uint64("count", s.activeRequest.Load()))
		}
	}()

	return s
}

// GetPing implements Connect PingPongServiceHandler.
func (p *pingPongConnectServer) GetPing(ctx context.Context, req *connect.Request[pbacme.GetPingRequest]) (*connect.Response[pbacme.PingResponse], error) {
	p.activeRequest.Inc()
	defer p.activeRequest.Dec()

	msg := req.Msg
	delay := time.Duration(msg.GetResponseDelayInMillis()) * time.Millisecond

	p.Logger.Info("GetPing Connect request",
		zap.String("from", msg.ClientId),
		zap.Duration("response_delay", delay),
		zap.String("message", msg.GetMessage()),
	)
	defer p.Logger.Info("GetPing Connect response", zap.String("from", msg.ClientId))

	if delay > 0 {
		time.Sleep(delay)
	}

	return connect.NewResponse(&pbacme.PingResponse{
		ServerId: p.ID,
		Message:  fmt.Sprintf("Connect Pong from %s (message from %s - %q)", p.ID, msg.ClientId, msg.GetMessage()),
	}), nil
}

// StreamPing implements Connect PingPongServiceHandler.
func (p *pingPongConnectServer) StreamPing(ctx context.Context, req *connect.Request[pbacme.StreamPingRequest], stream *connect.ServerStream[pbacme.PingResponse]) error {
	p.activeRequest.Inc()
	defer p.activeRequest.Dec()

	msg := req.Msg
	delay := time.Duration(msg.GetResponseDelayInMillis()) * time.Millisecond
	if delay <= 0 {
		delay = 250 * time.Millisecond
	}

	terminatesAfter := time.Duration(msg.GetTerminatesAfterMillis()) * time.Millisecond
	if terminatesAfter <= 0 {
		terminatesAfter = 10 * time.Second // Default to 10 seconds
	}

	p.Logger.Info("StreamPing Connect request",
		zap.String("from", msg.ClientId),
		zap.Duration("response_delay", delay),
		zap.Duration("terminates_after", terminatesAfter),
		zap.String("message", msg.GetMessage()),
	)
	defer p.Logger.Info("StreamPing Connect terminated", zap.String("from", msg.ClientId))

	terminatesTimer := time.NewTimer(terminatesAfter)
	defer terminatesTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			p.Logger.Info("StreamPing Connect context done", zap.Error(ctx.Err()))
			return nil
		case <-terminatesTimer.C:
			p.Logger.Info("StreamPing Connect terminated from after delay")
			return nil
		default:
			if delay > 0 {
				time.Sleep(delay)
			}

			response := &pbacme.PingResponse{
				ServerId: p.ID,
				Message:  fmt.Sprintf("Connect Stream Pong from %s (message from %s - %q)", p.ID, msg.ClientId, msg.GetMessage()),
			}

			if err := stream.Send(response); err != nil {
				p.Logger.Error("Failed to send Connect stream response", zap.Error(err))
				return err
			}
		}
	}
}

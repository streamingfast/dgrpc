package pbacme

import (
	context "context"
	"fmt"
	"math"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var _ PingPongServiceServer = &pingPongServiceServerImpl{}

type pingPongServiceServerImpl struct {
	UnimplementedPingPongServiceServer

	ID     string
	Logger *zap.Logger

	activeRequest *atomic.Uint64
}

func NewPingPongServiceServerImpl(id string, logger *zap.Logger) PingPongServiceServer {
	s := &pingPongServiceServerImpl{
		ID:            id,
		Logger:        logger,
		activeRequest: atomic.NewUint64(0),
	}

	// In a real world scenario, you would tie this to a proper metrics system
	// and tie it to a proper shutdown mechanism so that the goroutine and timer are stopped
	// properly
	go func() {
		for range time.Tick(5 * time.Second) {
			logger.Info("Active requests", zap.Uint64("count", s.activeRequest.Load()))
		}
	}()

	return s
}

// GetPing implements PingPongServer.
func (p *pingPongServiceServerImpl) GetPing(ctx context.Context, req *GetPingRequest) (*PingResponse, error) {
	p.activeRequest.Inc()
	defer p.activeRequest.Dec()

	delay := req.GetResponseDelay()

	p.Logger.Info("GetPing request", zap.String("from", req.ClientId), zap.Duration("response_delay", delay), zap.String("message", req.GetMessage()))
	defer p.Logger.Info("GetPing response", zap.String("from", req.ClientId))

	if int(delay) > 0 {
		time.Sleep(delay)
	}

	return &PingResponse{
		ServerId: p.ID,
		Message:  fmt.Sprintf("Pong from %s (message from %s - %q)", p.ID, req.ClientId, req.GetMessage()),
	}, nil
}

// StreamPing implements PingPongServer.
func (p *pingPongServiceServerImpl) StreamPing(req *StreamPingRequest, sender PingPongService_StreamPingServer) error {
	p.activeRequest.Inc()
	defer p.activeRequest.Dec()

	delay := req.GetResponseDelay()
	if int(delay) <= 0 {
		delay = 250 * time.Millisecond
	}

	terminatesAfter := req.GetTerminatesAfter()
	if int(terminatesAfter) != 0 {
		terminatesAfter = time.Duration(math.MaxInt64)
	}

	p.Logger.Info("StreamPing request", zap.String("from", req.ClientId), zap.Duration("response_delay", delay), zap.Duration("terminates_after", terminatesAfter), zap.String("message", req.GetMessage()))
	defer p.Logger.Info("StreamPing terminated", zap.String("from", req.ClientId))

	terminatesTimer := time.NewTimer(terminatesAfter)
	defer terminatesTimer.Stop()

	for {
		select {
		case <-sender.Context().Done():
			p.Logger.Info("StreamPing context done", zap.NamedError("err", context.Cause(sender.Context())))
			return nil
		case <-terminatesTimer.C:
			p.Logger.Info("StreamPing terminated from after delay")
			return nil
		default:
			if int(delay) > 0 {
				time.Sleep(delay)
			}

			sender.SendMsg(&PingResponse{
				ServerId: p.ID,
				Message:  fmt.Sprintf("Pong from %s (message from %s - %q)", p.ID, req.ClientId, req.GetMessage()),
			})
		}
	}
}

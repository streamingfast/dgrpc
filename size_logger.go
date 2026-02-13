package dgrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/streamingfast/logging/zapx"
	"go.uber.org/zap"
	"google.golang.org/grpc/stats"
)

func NewSizeLoggingHandler(messageLimit int, logger *zap.Logger) *SizeLoggingHandler {
	return &SizeLoggingHandler{
		messageLimit: messageLimit,
		logger:       logger,
	}
}

type SizeLoggingHandler struct {
	messageLimit int
	logger       *zap.Logger

	messageCount      int
	timeStart         time.Time
	uncompressedBytes int
	compressedBytes   int
	wireBytes         int
	lastReceivedTime  time.Time
	waitToReceive     time.Duration
}

func (h *SizeLoggingHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	h.logger.Info("gRPC client RPC started", zap.String("method", info.FullMethodName))
	return ctx
}

func (h *SizeLoggingHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	if inPayload, ok := rs.(*stats.InPayload); ok && inPayload != nil {
		if h.messageCount == 0 {
			h.timeStart = time.Now()
		}
		h.waitToReceive += time.Since(h.lastReceivedTime)
		h.lastReceivedTime = time.Now()
		h.messageCount++
		h.uncompressedBytes += inPayload.Length
		h.compressedBytes += inPayload.CompressedLength
		h.wireBytes += inPayload.WireLength

		if h.messageCount == h.messageLimit {
			secs := time.Since(h.timeStart).Seconds()
			messagesPerSecond := float64(h.messageCount) / secs
			compressedPercentage := 100.0 - (float64(h.compressedBytes) / float64(h.uncompressedBytes) * 100.0)

			h.logger.Info(
				"grpc io stats",
				zap.Float64("msg_sec", messagesPerSecond),
				zapx.HumanDuration("duration", time.Since(h.timeStart)),
				zapx.HumanDuration("wait_to_receive", h.waitToReceive),
				zap.String("compression_ratio", fmt.Sprintf("%.2f%%", compressedPercentage)),
				zap.String("uncompressed", humanize.Bytes(uint64(h.uncompressedBytes))),
				zap.String("compressed", humanize.Bytes(uint64(h.compressedBytes))),
				zap.Int("uncompressed_bytes", h.uncompressedBytes),
				zap.Int("compressed_bytes", h.compressedBytes),
				zap.Bool("keep", false),
			)

			h.timeStart = time.Now()
			h.messageCount = 0
			h.uncompressedBytes = 0
			h.compressedBytes = 0
			h.wireBytes = 0
			h.waitToReceive = 0
		}
	}
}

func (h *SizeLoggingHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	return ctx
}

func (h *SizeLoggingHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
}

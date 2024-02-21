package dgrpc

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/streamingfast/logging"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/grpclog"
)

var grpcSetLogger SettableLoggerV2
var zlogGRPC, tracerGRPC = logging.PackageLogger("grpc", "google.golang.org/grpc", logging.LoggerDefaultLevel(zapcore.ErrorLevel))

func init() {
	// The `settableLoggerV2` provides a way to replace the default gRPC logger with a custom
	// one dynamically. We re-initialize it here to ensure that we can replace the logger
	// and then consumers of the library can replace it with their own logger later on.
	grpcSetLogger = newSettableLoggerV2()
	grpclog.SetLoggerV2(grpcSetLogger)

	// We replace the default gRPC logger with our own logger if environment variable is set
	if os.Getenv("GRPC_REGISTER_ZAP_LOGGER") != "" {
		SetGRPCLogger(zlogGRPC, tracerGRPC)
	}
}

// SetGRPCLogger replaces the grpc_log.LoggerV2 with the provided logger. The gRPC level
// logging verbosity which is a value between 0 and 3 (both end inclusive) is inferred based
// on the logger/tracer level. If logger is in INFO or higher (WARN, ERROR, DPANIC, PANIC, FATAL),
// the verbosity is set to 0. If the logger is in DEBUG, the verbosity is set to 1. If the logger
// is in TRACE (tracer.Enabled()), the verbosity is set to 3 (full verbosity).
//
// Use [SetGRPCLoggerWithVerbosity] to control the verbosity manually.
//
// If you don't have a 'tracer', you can pass nil here.
func SetGRPCLogger(logger *zap.Logger, tracer logging.Tracer) {
	SetGRPCLoggerWithVerbosity(logger, tracer, -1)
}

// SetGrpcLoggerV2WithVerbosity replaces the grpc_.LoggerV2 with the provided logger and verbosity.
// It can be used even when grpc infrastructure was initialized.
//
// If you don't have a 'tracer', you can pass nil here.
func SetGRPCLoggerWithVerbosity(logger *zap.Logger, tracer logging.Tracer, verbosity int) {
	zgl := &zapGrpcLoggerV2{
		logger:    logger,
		tracer:    tracer,
		verbosity: verbosity,
	}

	grpcSetLogger.Set(zgl)
}

type zapGrpcLoggerV2 struct {
	logger    *zap.Logger
	tracer    logging.Tracer
	verbosity int
}

func (l *zapGrpcLoggerV2) Info(args ...interface{}) {
	// By default we are in debug even if grpc logger is set to info as we assume anything going out of grpclog in Info is a debug for us
	l.writeArgs(zapcore.DebugLevel, args...)
}

func (l *zapGrpcLoggerV2) Infoln(args ...interface{}) {
	// By default we are in debug even if grpc logger is set to info as we assume anything going out of grpclog in Info is a debug for us
	l.writeArgs(zapcore.DebugLevel, args...)
}

func (l *zapGrpcLoggerV2) Infof(format string, args ...interface{}) {
	// By default we are in debug even if grpc logger is set to info as we assume anything going out of grpclog in Info is a debug for us
	l.writeArgs(zapcore.DebugLevel, fmt.Sprintf(format, args...))
}

func (l *zapGrpcLoggerV2) Warning(args ...interface{}) {
	l.writeArgs(zapcore.WarnLevel, args...)
}

func (l *zapGrpcLoggerV2) Warningln(args ...interface{}) {
	l.writeArgs(zapcore.WarnLevel, args...)
}

func (l *zapGrpcLoggerV2) Warningf(format string, args ...interface{}) {
	l.writeArgs(zapcore.WarnLevel, fmt.Sprintf(format, args...))
}

func (l *zapGrpcLoggerV2) Error(args ...interface{}) {
	l.writeArgs(zapcore.ErrorLevel, args...)
}

func (l *zapGrpcLoggerV2) Errorln(args ...interface{}) {
	l.writeArgs(zapcore.ErrorLevel, args...)
}

func (l *zapGrpcLoggerV2) Errorf(format string, args ...interface{}) {
	l.writeArgs(zapcore.ErrorLevel, fmt.Sprintf(format, args...))
}

func (l *zapGrpcLoggerV2) Fatal(args ...interface{}) {
	l.writeArgs(zapcore.FatalLevel, args...)
}

func (l *zapGrpcLoggerV2) Fatalln(args ...interface{}) {
	l.writeArgs(zapcore.FatalLevel, args...)
}

func (l *zapGrpcLoggerV2) Fatalf(format string, args ...interface{}) {
	l.writeArgs(zapcore.FatalLevel, fmt.Sprintf(format, args...))
}

func (l *zapGrpcLoggerV2) V(level int) bool {
	verbosity := l.verbosity
	if verbosity < 0 {
		if l.tracer != nil && l.tracer.Enabled() {
			verbosity = 3
		} else if l.logger.Core().Enabled(zapcore.DebugLevel) {
			verbosity = 1
		} else {
			verbosity = 0
		}
	}

	return l.verbosity <= level
}

type depthArgument int

// DepthLogger implementation without referencing it directly so that we remaing compatible
// if the grpc library changes the signature of the logger.
// InfoDepth logs to INFO log at the specified depth. Arguments are handled in the manner of fmt.Println.
func (l *zapGrpcLoggerV2) InfoDepth(depth int, args ...any) {
	l.Info(append([]any{depthArgument(depth)}, args...))
}

// WarningDepth logs to WARNING log at the specified depth. Arguments are handled in the manner of fmt.Println.
func (l *zapGrpcLoggerV2) WarningDepth(depth int, args ...any) {
	l.Warning(append([]any{depthArgument(depth)}, args...))
}

// ErrorDepth logs to ERROR log at the specified depth. Arguments are handled in the manner of fmt.Println.
func (l *zapGrpcLoggerV2) ErrorDepth(depth int, args ...any) {
	l.Error(append([]any{depthArgument(depth)}, args...))
}

// FatalDepth logs to FATAL log at the specified depth. Arguments are handled in the manner of fmt.Println.
func (l *zapGrpcLoggerV2) FatalDepth(depth int, args ...any) {
	l.Fatal(append([]any{depthArgument(depth)}, args...))
}

var inSquareBrackets = regexp.MustCompile(`^\s*\[(.*?)\]\s*(.*)?$`)

func (l *zapGrpcLoggerV2) writeArgs(level zapcore.Level, args ...interface{}) {
	if !l.logger.Core().Enabled(level) {
		// Avoid working if the level is not enabled
		return
	}

	var rest []string
	var modules []string
	var fields []zap.Field

	for _, arg := range args {
		switch v := arg.(type) {
		case depthArgument:
			fields = append(fields, zap.Int("depth", int(v)))

		case string:
			if matches := inSquareBrackets.FindStringSubmatch(v); len(matches) == 3 {
				modules = append(modules, matches[1])
				if remaining := strings.TrimSpace(matches[2]); remaining != "" {
					rest = append(rest, remaining)
				}

				continue
			}
		}

		rest = append(rest, argToString(arg))
	}

	if len(modules) > 0 {
		// We put "module" first to present it first
		fields = append([]zap.Field{zap.Stringer("module", newJoinableString(modules, "/"))}, fields...)
	}

	// It seems that a lot of messages are prefixed with "grpc: ", we remove it
	msg := strings.TrimPrefix(strings.Join(rest, " "), "grpc: ")
	l.logger.Check(level, msg).Write(fields...)
}

func newJoinableString(strings []string, separator string) joinableString {
	return joinableString{
		strings:   strings,
		separator: separator,
	}
}

type joinableString struct {
	strings   []string
	separator string
}

func (j joinableString) String() string {
	return strings.Join(j.strings, j.separator)
}

func argToString(arg any) string {
	switch arg := arg.(type) {
	case string:
		return arg

	case fmt.Stringer:
		return arg.String()

	case error:
		return arg.Error()

	default:
		return fmt.Sprintf("%s", arg)
	}
}

// SettableLoggerV2 is thread-safe.
type SettableLoggerV2 interface {
	grpclog.LoggerV2
	// Sets given logger as the underlying implementation.
	Set(loggerv2 grpclog.LoggerV2)
	// Sets `discard` logger as the underlying implementation.
	Reset()
}

func newSettableLoggerV2() *settableLoggerV2 {
	return &settableLoggerV2{
		log: &atomic.Value{},
	}
}

type settableLoggerV2 struct {
	log *atomic.Value
}

func (s *settableLoggerV2) Set(log grpclog.LoggerV2) {
	s.log.Store(log)
}

func (s *settableLoggerV2) Reset() {
	s.Set(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
}

func (s *settableLoggerV2) get() grpclog.LoggerV2 {
	if v := s.log.Load(); v == nil {
		return nil
	} else {
		return v.(grpclog.LoggerV2)
	}
}

func (s *settableLoggerV2) Info(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Info(args...)
	}
}

func (s *settableLoggerV2) Infoln(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Infoln(args...)
	}
}

func (s *settableLoggerV2) Infof(format string, args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Infof(format, args...)
	}
}

func (s *settableLoggerV2) Warning(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Warning(args...)
	}
}

func (s *settableLoggerV2) Warningln(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Warningln(args...)
	}
}

func (s *settableLoggerV2) Warningf(format string, args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Warningf(format, args...)
	}
}

func (s *settableLoggerV2) Error(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Error(args...)
	}
}

func (s *settableLoggerV2) Errorln(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Errorln(args...)
	}
}

func (s *settableLoggerV2) Errorf(format string, args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Errorf(format, args...)
	}
}

func (s *settableLoggerV2) Fatal(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Fatal(args...)
	}
}

func (s *settableLoggerV2) Fatalln(args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Fatalln(args...)
	}
}

func (s *settableLoggerV2) Fatalf(format string, args ...interface{}) {
	if logger := s.get(); logger != nil {
		logger.Fatalf(format, args...)
	}
}

func (s *settableLoggerV2) V(l int) bool {
	if logger := s.get(); logger != nil {
		return logger.V(l)
	} else {
		return false
	}
}

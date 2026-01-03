package mtcrtc

import (
	"fmt"
	"log/slog"

	"github.com/pion/logging"
)

// ref https://github.com/pion/webrtc/blob/master/examples/custom-logger/main.go

type slogLogger struct {
	subsystem string
}

func (l slogLogger) Trace(msg string) { slog.Debug(msg, "subsystem", l.subsystem, "level", "trace") }
func (l slogLogger) Tracef(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...), "subsystem", l.subsystem, "level", "trace")
}
func (l slogLogger) Debug(msg string) { slog.Debug(msg, "subsystem", l.subsystem) }
func (l slogLogger) Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...), "subsystem", l.subsystem)
}
func (l slogLogger) Info(msg string) { slog.Info(msg, "subsystem", l.subsystem) }
func (l slogLogger) Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...), "subsystem", l.subsystem)
}
func (l slogLogger) Warn(msg string) { slog.Warn(msg, "subsystem", l.subsystem) }
func (l slogLogger) Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...), "subsystem", l.subsystem)
}
func (l slogLogger) Error(msg string) { slog.Error(msg, "subsystem", l.subsystem) }
func (l slogLogger) Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...), "subsystem", l.subsystem)
}

type slogLoggerFactory struct{}

func (f slogLoggerFactory) NewLogger(subsystem string) logging.LeveledLogger {
	return slogLogger{subsystem: subsystem}
}

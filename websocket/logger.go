package mtcws

import (
	"fmt"
	"log/slog"

	"github.com/lesismal/nbio/logging"
)

type slogLogger struct{}

func (l *slogLogger) SetLevel(level int) {}

func (l *slogLogger) Debug(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...), "module", "nbio")
}

func (l *slogLogger) Info(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...), "module", "nbio")
}

func (l *slogLogger) Warn(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...), "module", "nbio")
}

func (l *slogLogger) Error(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...), "module", "nbio")
}

func InitNbioLogger() {
	logging.SetLogger(&slogLogger{})
}

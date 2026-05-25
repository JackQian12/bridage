package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a zap logger appropriate for the given log level string.
// Valid values: "debug", "info", "warn", "error". Defaults to "info".
func New(level string) (*zap.Logger, error) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = zapcore.InfoLevel
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(l)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg.Build()
}

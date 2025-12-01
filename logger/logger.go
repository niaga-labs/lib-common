package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new Zap logger instance
func NewLogger(env string) (*zap.Logger, error) {
	var config zap.Config

	if env == "production" {
		// Production: JSON output, INFO level
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		// Development: Console output, DEBUG level
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := config.Build(
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// NewSugaredLogger creates a new sugared Zap logger instance
func NewSugaredLogger(env string) (*zap.SugaredLogger, error) {
	logger, err := NewLogger(env)
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

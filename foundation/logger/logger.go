package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New constructs a SuggaredLogger that writes to stderr and provides human readable timestamps.
func New(service string, outputPaths ...string) (*zap.SugaredLogger, error) {
	// Create a default config for development.
	config := zap.NewProductionConfig()

	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.DisableStacktrace = true
	config.InitialFields = map[string]interface{}{"service": service}

	config.OutputPaths = []string{"stdout"}
	if outputPaths != nil {
		config.OutputPaths = outputPaths
	}

	// Create a logger for the service.
	logger, err := config.Build(zap.WithCaller(true))
	if err != nil {
		return nil, err
	}

	// Return a SugaredLogger.
	return logger.Sugar(), nil
}

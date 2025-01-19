package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ComponentType represents the type of component doing the logging
type ComponentType string

const (
	ComponentGenerator ComponentType = "generator"
	ComponentController ComponentType = "controller"
)

// Logger wraps zap logger with component context
type Logger struct {
	*zap.Logger
}

// NewLogger creates a new structured logger for a component
func NewLogger(component string, compType ComponentType) *Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	// Add component fields
	logger = logger.With(
		zap.String("component", component),
		zap.String("type", string(compType)),
	)

	return &Logger{Logger: logger}
}

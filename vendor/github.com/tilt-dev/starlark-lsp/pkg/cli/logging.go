package cli

import (
	"fmt"

	"go.uber.org/zap"
)

func NewLogger() (logger *zap.Logger, cleanup func()) {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = logLevel
	cfg.Development = false
	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Errorf("failed to initialize logger: %v", err))
	}

	cleanup = func() {
		_ = logger.Sync()
	}
	return logger, cleanup
}

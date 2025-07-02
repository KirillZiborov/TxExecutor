// Package logging provides logging utilities for client and server.
// It leverages the zap library to offer structured and performant logging.
package logging

import (
	"go.uber.org/zap"
)

// Sugar is a globally accessible SugaredLogger instance.
// It provides a more ergonomic API for logging compared to the base Zap logger.
var Sugar zap.SugaredLogger

// Initialize sets up the global SugaredLogger using Zap's development configuration.
// It must be called before using Sugar. If initialization fails, the function returns an error.
func Initialize() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	Sugar = *logger.Sugar()
	return nil
}

func init() {
	if err := Initialize(); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
}

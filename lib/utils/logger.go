package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SetupLogger() *zap.SugaredLogger {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger := zap.Must(cfg.Build())
	// Packages without an injected logger (changeset, hooks, plugins) log
	// through zap.S(); point it at the application logger.
	zap.ReplaceGlobals(logger)
	sugar := logger.Sugar()

	return sugar
}

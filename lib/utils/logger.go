package utils

import "go.uber.org/zap"

func SetupLogger() *zap.SugaredLogger {
	logger, _ := zap.NewProduction()
	sugar := logger.Sugar()

	return sugar
}

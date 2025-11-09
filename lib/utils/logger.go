package utils

import "go.uber.org/zap"

func SetupLogger() *zap.SugaredLogger {
	logger, _ := zap.NewProduction()
	logger = zap.Must(zap.NewDevelopment())
	sugar := logger.Sugar()

	return sugar
}

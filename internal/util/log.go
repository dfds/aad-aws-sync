package util

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func InitializeLogger() {
	//Logger, _ = zap.NewDevelopment()

	devConf := zap.NewDevelopmentConfig()
	Logger, _ = devConf.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}

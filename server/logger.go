package server

import "go.uber.org/zap"

var logger *zap.Logger

func init() {
	logger, _ = zap.NewProductionConfig().Build()
}

func SetLogger(l *zap.Logger) error {
	if logger != nil {
		logger = l
	}
	return nil
}

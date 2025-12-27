package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/samber/do"
	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/serroba/web-demo-go/internal/container"
	"go.uber.org/zap"
)

func main() {
	opts := &container.Options{
		RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),
		LogFormat: getEnv("LOG_FORMAT", "console"),
	}

	injector := do.New()
	do.ProvideValue(injector, opts)
	container.LoggerPackage(injector)
	container.RedisPackage(injector)
	container.AnalyticsConsumerPackage(injector)

	logger := do.MustInvoke[*zap.Logger](injector)
	consumer := do.MustInvoke[*analytics.Consumer](injector)

	ctx, cancel := context.WithCancel(context.Background())

	if err := consumer.Start(ctx); err != nil {
		logger.Fatal("failed to start consumer", zap.Error(err))
	}

	logger.Info("analytics consumer started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down")
	cancel()

	if err := injector.Shutdown(); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}

	logger.Info("shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultValue
}

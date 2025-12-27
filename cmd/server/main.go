package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
	"github.com/serroba/web-demo-go/internal/container"
	"go.uber.org/zap"
)

func registerPackages(injector *do.Injector, options *container.Options) {
	do.ProvideValue(injector, options)
	container.LoggerPackage(injector)
	container.RedisPackage(injector)
	container.PostgresPackage(injector)
	container.RepositoryPackage(injector)
	container.RateLimitPackage(injector)
	container.PublisherGroupPackage(injector)
	container.HTTPPackage(injector)
}

func main() {
	cli := humacli.New(func(hooks humacli.Hooks, options *container.Options) {
		injector := do.New()
		registerPackages(injector, options)

		logger := do.MustInvoke[*zap.Logger](injector)

		var server *http.Server

		hooks.OnStart(func() {
			router := do.MustInvoke[*chi.Mux](injector)

			// Invoke API to trigger route registration
			_ = do.MustInvoke[huma.API](injector)

			server = &http.Server{
				Addr:              fmt.Sprintf(":%d", options.Port),
				Handler:           router,
				ReadHeaderTimeout: 10 * time.Second,
			}

			logger.Info("server starting", zap.Int("port", options.Port))

			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal("server failed", zap.Error(err))
			}
		})

		hooks.OnStop(func() {
			logger.Info("shutting down")

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if server != nil {
				if err := server.Shutdown(ctx); err != nil {
					logger.Error("server shutdown error", zap.Error(err))
				}
			}

			if err := injector.Shutdown(); err != nil {
				logger.Error("service shutdown error", zap.Error(err))
			}

			logger.Info("shutdown complete")
		})
	})

	cli.Run()
}

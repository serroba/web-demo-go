package container

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor" // CBOR format support for huma
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/jaevor/go-nanoid"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"
	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/serroba/web-demo-go/internal/store"
)

type Options struct {
	Port       int    `default:"8888"           help:"Port to listen on"               short:"p"`
	CodeLength int    `default:"8"              help:"Length of generated short codes" short:"c"`
	RedisAddr  string `default:"localhost:6379" help:"Redis server address"            short:"r"`
}

func New(_ humacli.Hooks, options *Options) *do.Injector {
	injector := do.New()

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("URL Shortener", "1.0.0"))

	redisClient := redis.NewClient(&redis.Options{
		Addr: options.RedisAddr,
	})
	urlStore := store.NewRedisStore(redisClient)
	baseURL := fmt.Sprintf("http://localhost:%d", options.Port)

	codeGenerator, _ := nanoid.Standard(options.CodeLength)

	strategies := map[handlers.Strategy]handlers.ShortenerStrategy{
		handlers.StrategyToken: handlers.NewTokenStrategy(urlStore, codeGenerator),
		handlers.StrategyHash:  handlers.NewHashStrategy(urlStore, codeGenerator),
	}

	urlHandler := handlers.NewURLHandler(urlStore, baseURL, strategies)

	do.ProvideValue(injector, router)
	do.ProvideValue(injector, api)
	do.ProvideValue(injector, options)
	do.ProvideValue(injector, redisClient)

	handlers.RegisterRoutes(api, urlHandler)

	return injector
}

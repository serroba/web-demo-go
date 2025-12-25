package container

import (
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor" // CBOR format support for huma
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/jaevor/go-nanoid"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"
	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/serroba/web-demo-go/internal/health"
	"github.com/serroba/web-demo-go/internal/middleware"
	"github.com/serroba/web-demo-go/internal/ratelimit"
	"github.com/serroba/web-demo-go/internal/shortener"
	"github.com/serroba/web-demo-go/internal/store"
)

type Options struct {
	Port            int           `default:"8888"           help:"Port to listen on"    short:"p"`
	CodeLength      int           `default:"8"              help:"Short code length"    short:"c"`
	RedisAddr       string        `default:"localhost:6379" help:"Redis server address" short:"r"`
	RateLimitReqs   int64         `default:"100"            env:"RATE_LIMIT_REQUESTS"   help:"Requests per window"`
	RateLimitWindow time.Duration `default:"1m"             env:"RATE_LIMIT_WINDOW"     help:"Rate limit window"`
	RateLimitStore  string        `default:"memory"         env:"RATE_LIMIT_STORE"      help:"memory or redis"`
}

func New(_ humacli.Hooks, options *Options) *do.Injector {
	injector := do.New()

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("URL Shortener", "1.0.0"))

	redisClient := redis.NewClient(&redis.Options{
		Addr: options.RedisAddr,
	})

	// Set up rate limiting with configurable backend
	rateLimitStore := newRateLimitStore(options.RateLimitStore, redisClient)
	limiter := ratelimit.NewSlidingWindowLimiter(rateLimitStore, options.RateLimitReqs, options.RateLimitWindow)
	api.UseMiddleware(middleware.RateLimiter(api, limiter))

	urlStore := store.NewRedisStore(redisClient)
	baseURL := fmt.Sprintf("http://localhost:%d", options.Port)

	codeGenerator, _ := nanoid.Standard(options.CodeLength)

	strategies := map[handlers.Strategy]shortener.Strategy{
		handlers.StrategyToken: shortener.NewTokenStrategy(urlStore, codeGenerator),
		handlers.StrategyHash:  shortener.NewHashStrategy(urlStore, codeGenerator),
	}

	urlHandler := handlers.NewURLHandler(urlStore, baseURL, strategies)
	healthHandler := health.NewHandler(health.NewRedisChecker(redisClient))

	do.ProvideValue(injector, router)
	do.ProvideValue(injector, api)
	do.ProvideValue(injector, options)
	do.ProvideValue(injector, redisClient)

	handlers.RegisterRoutes(api, urlHandler)
	health.RegisterRoutes(api, healthHandler)

	return injector
}

func newRateLimitStore(storeType string, redisClient *redis.Client) ratelimit.Store {
	switch storeType {
	case "redis":
		return store.NewRateLimitRedisStore(redisClient)
	default:
		return store.NewRateLimitMemoryStore()
	}
}

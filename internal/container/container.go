package container

import (
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor" // CBOR format support for huma
	"github.com/go-chi/chi/v5"
	"github.com/jaevor/go-nanoid"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"
	"github.com/serroba/web-demo-go/internal/analytics"
	analyticsstore "github.com/serroba/web-demo-go/internal/analytics/store"
	"github.com/serroba/web-demo-go/internal/cache"
	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/serroba/web-demo-go/internal/health"
	"github.com/serroba/web-demo-go/internal/messaging"
	"github.com/serroba/web-demo-go/internal/middleware"
	"github.com/serroba/web-demo-go/internal/ratelimit"
	ratelimitstore "github.com/serroba/web-demo-go/internal/ratelimit/store"
	"github.com/serroba/web-demo-go/internal/shortener"
	"github.com/serroba/web-demo-go/internal/store"
	"go.uber.org/zap"
)

type Options struct {
	Port             int           `default:"8888"           help:"Port to listen on"  short:"p"`
	CodeLength       int           `default:"8"              help:"Short code length"  short:"c"`
	RedisAddr        string        `default:"localhost:6379" help:"Redis address"      short:"r"`
	RateLimitReqs    int64         `default:"100"            env:"RATE_LIMIT_REQUESTS" help:"Requests per window"`
	RateLimitWindow  time.Duration `default:"1m"             env:"RATE_LIMIT_WINDOW"   help:"Rate limit window"`
	RateLimitStore   string        `default:"memory"         env:"RATE_LIMIT_STORE"    help:"memory or redis"`
	CacheSize        int           `default:"1000"           env:"CACHE_SIZE"          help:"LRU cache size (0=off)"`
	LogFormat        string        `default:"console"        env:"LOG_FORMAT"          help:"console or json"`
	TopicURLCreated  string        `default:"url.created"    env:"TOPIC_URL_CREATED"   help:"URL created topic"`
	TopicURLAccessed string        `default:"url.accessed"   env:"TOPIC_URL_ACCESSED"  help:"URL accessed topic"`
	ConsumerGroup    string        `default:"analytics"      env:"CONSUMER_GROUP"      help:"Consumer group name"`
}

// LoggerPackage provides the zap logger.
func LoggerPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*zap.Logger, error) {
		opts := do.MustInvoke[*Options](i)

		if opts.LogFormat == "json" {
			return zap.NewProduction()
		}

		return zap.NewDevelopment()
	})
}

// RedisPackage provides the Redis client.
func RedisPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*redis.Client, error) {
		opts := do.MustInvoke[*Options](i)

		return redis.NewClient(&redis.Options{
			Addr: opts.RedisAddr,
		}), nil
	})
}

// RepositoryPackage provides the URL repository with optional caching.
func RepositoryPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (shortener.Repository, error) {
		opts := do.MustInvoke[*Options](i)
		redisClient := do.MustInvoke[*redis.Client](i)

		var repo shortener.Repository = store.NewRedisStore(redisClient)

		if opts.CacheSize > 0 {
			repo = store.NewCachedRepository(repo, cache.New(opts.CacheSize))
		}

		return repo, nil
	})
}

// RateLimitPackage provides the rate limit store.
func RateLimitPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (ratelimit.Store, error) {
		opts := do.MustInvoke[*Options](i)
		redisClient := do.MustInvoke[*redis.Client](i)

		switch opts.RateLimitStore {
		case "redis":
			return ratelimitstore.NewRedis(redisClient), nil
		default:
			return ratelimitstore.NewMemory(), nil
		}
	})
}

// PublisherGroupPackage provides the publisher group for event publishing.
func PublisherGroupPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*messaging.PublisherGroup, error) {
		redisClient := do.MustInvoke[*redis.Client](i)

		publisher, err := redisstream.NewPublisher(
			redisstream.PublisherConfig{
				Client: redisClient,
			},
			watermill.NopLogger{},
		)
		if err != nil {
			return nil, err
		}

		return messaging.NewPublisherGroup(publisher), nil
	})
}

// ConsumerGroupPackage provides the consumer group with all registered consumers.
func ConsumerGroupPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*messaging.ConsumerGroup, error) {
		opts := do.MustInvoke[*Options](i)
		redisClient := do.MustInvoke[*redis.Client](i)
		logger := do.MustInvoke[*zap.Logger](i)

		subscriber, err := redisstream.NewSubscriber(
			redisstream.SubscriberConfig{
				Client:        redisClient,
				ConsumerGroup: opts.ConsumerGroup,
			},
			watermill.NopLogger{},
		)
		if err != nil {
			return nil, err
		}

		analyticsStore := analyticsstore.NewNoop(logger)
		group := messaging.NewConsumerGroup(subscriber, logger)

		// Register analytics consumers
		group.Add(messaging.NewConsumer(
			subscriber,
			opts.TopicURLCreated,
			analyticsStore.SaveURLCreated,
			logger,
		))

		group.Add(messaging.NewConsumer(
			subscriber,
			opts.TopicURLAccessed,
			analyticsStore.SaveURLAccessed,
			logger,
		))

		return group, nil
	})
}

// HTTPPackage provides the router, API, and registers routes.
func HTTPPackage(i *do.Injector) {
	do.Provide(i, func(_ *do.Injector) (*chi.Mux, error) {
		return chi.NewMux(), nil
	})

	do.Provide(i, func(i *do.Injector) (huma.API, error) {
		router := do.MustInvoke[*chi.Mux](i)
		opts := do.MustInvoke[*Options](i)
		logger := do.MustInvoke[*zap.Logger](i)
		redisClient := do.MustInvoke[*redis.Client](i)
		urlStore := do.MustInvoke[shortener.Repository](i)
		rateLimitStore := do.MustInvoke[ratelimit.Store](i)
		publisherGroup := do.MustInvoke[*messaging.PublisherGroup](i)

		api := humachi.New(router, huma.DefaultConfig("URL Shortener", "1.0.0"))

		// Set up middleware
		api.UseMiddleware(middleware.RequestMeta(api))

		limiter := ratelimit.NewSlidingWindowLimiter(rateLimitStore, opts.RateLimitReqs, opts.RateLimitWindow)
		api.UseMiddleware(middleware.RateLimiter(api, limiter))

		// Set up handlers
		baseURL := fmt.Sprintf("http://localhost:%d", opts.Port)
		codeGenerator, _ := nanoid.Standard(opts.CodeLength)

		strategies := map[handlers.Strategy]shortener.Strategy{
			handlers.StrategyToken: shortener.NewTokenStrategy(urlStore, codeGenerator),
			handlers.StrategyHash:  shortener.NewHashStrategy(urlStore, codeGenerator),
		}

		pub := publisherGroup.Publisher()
		urlHandler := handlers.NewURLHandler(
			urlStore,
			baseURL,
			strategies,
			messaging.NewPublishFunc[analytics.URLCreatedEvent](pub, opts.TopicURLCreated),
			messaging.NewPublishFunc[analytics.URLAccessedEvent](pub, opts.TopicURLAccessed),
			logger,
		)
		healthHandler := health.NewHandler(health.NewRedisChecker(redisClient))

		// Register routes
		handlers.RegisterRoutes(api, urlHandler)
		health.RegisterRoutes(api, healthHandler)

		return api, nil
	})
}

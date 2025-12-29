package container

import (
	"context"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor" // CBOR format support for huma
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	Port             int           `default:"8888"           help:"Port to listen on" short:"p"`
	CodeLength       int           `default:"8"              help:"Short code length" short:"c"`
	RedisAddr        string        `default:"localhost:6379" help:"Redis address"     short:"r"`
	DatabaseURL      string        `env:"DATABASE_URL"       help:"PostgreSQL URL"    required:""`
	RateLimitStore   string        `default:"memory"         env:"RATE_LIMIT_STORE"   help:"memory or redis"`
	CacheSize        int           `default:"1000"           env:"CACHE_SIZE"         help:"LRU cache size (0=off)"`
	CacheTTL         time.Duration `default:"1h"             env:"CACHE_TTL"          help:"Redis cache TTL"`
	LogFormat        string        `default:"console"        env:"LOG_FORMAT"         help:"console or json"`
	TopicURLCreated  string        `default:"url.created"    env:"TOPIC_URL_CREATED"  help:"URL created topic"`
	TopicURLAccessed string        `default:"url.accessed"   env:"TOPIC_URL_ACCESSED" help:"URL accessed topic"`
	ConsumerGroup    string        `default:"analytics"      env:"CONSUMER_GROUP"     help:"Consumer group name"`

	// Rate limit configuration per scope
	RateLimitGlobalPerDay   int64 `default:"1000000" env:"RATE_LIMIT_GLOBAL_DAY"   help:"Global requests per day"`
	RateLimitReadPerMinute  int64 `default:"100000"  env:"RATE_LIMIT_READ_MINUTE"  help:"Read requests per minute"`
	RateLimitWritePerMinute int64 `default:"10"      env:"RATE_LIMIT_WRITE_MINUTE" help:"Write requests per minute"`
	RateLimitWritePerHour   int64 `default:"100"     env:"RATE_LIMIT_WRITE_HOUR"   help:"Write requests per hour"`
	RateLimitWritePerDay    int64 `default:"500"     env:"RATE_LIMIT_WRITE_DAY"    help:"Write requests per day"`
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

// RedisClient wraps redis.Client to implement Shutdownable for do.Injector.
type RedisClient struct {
	*redis.Client
}

// Shutdown implements do.Shutdownable.
func (r *RedisClient) Shutdown() error {
	if r.Client != nil {
		return r.Close()
	}

	return nil
}

// RedisPackage provides the Redis client.
func RedisPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*RedisClient, error) {
		opts := do.MustInvoke[*Options](i)

		return &RedisClient{
			Client: redis.NewClient(&redis.Options{
				Addr: opts.RedisAddr,
			}),
		}, nil
	})
}

// PostgresPool wraps pgxpool.Pool to implement Shutdownable for do.Injector.
type PostgresPool struct {
	*pgxpool.Pool
}

// Shutdown implements do.Shutdownable.
func (p *PostgresPool) Shutdown() error {
	if p.Pool != nil {
		p.Close()
	}

	return nil
}

// PostgresPackage provides the PostgreSQL connection pool.
func PostgresPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*PostgresPool, error) {
		opts := do.MustInvoke[*Options](i)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, opts.DatabaseURL)
		if err != nil {
			return nil, err
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()

			return nil, err
		}

		return &PostgresPool{Pool: pool}, nil
	})
}

// RepositoryPackage provides the URL repository with Redis caching over PostgreSQL.
func RepositoryPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (shortener.Repository, error) {
		opts := do.MustInvoke[*Options](i)
		pool := do.MustInvoke[*PostgresPool](i)
		redisClient := do.MustInvoke[*RedisClient](i)

		// PostgreSQL as source of truth
		postgresStore := store.NewPostgresStore(pool.Pool)

		// Redis cache layer with configurable TTL
		var repo shortener.Repository = store.NewRedisCacheRepository(postgresStore, redisClient.Client, opts.CacheTTL)

		// Optional in-memory LRU cache on top
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
		redisClient := do.MustInvoke[*RedisClient](i)

		switch opts.RateLimitStore {
		case "redis":
			return ratelimitstore.NewRedis(redisClient.Client), nil
		default:
			return ratelimitstore.NewMemory(), nil
		}
	})
}

// PublisherGroupPackage provides the publisher group for event publishing.
func PublisherGroupPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*messaging.PublisherGroup, error) {
		redisClient := do.MustInvoke[*RedisClient](i)

		publisher, err := redisstream.NewPublisher(
			redisstream.PublisherConfig{
				Client: redisClient.Client,
			},
			watermill.NopLogger{},
		)
		if err != nil {
			return nil, err
		}

		return messaging.NewPublisherGroup(publisher), nil
	})
}

// AnalyticsStorePackage provides the analytics store for persisting events.
func AnalyticsStorePackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (analytics.Store, error) {
		pool := do.MustInvoke[*PostgresPool](i)

		return analyticsstore.NewPostgres(pool.Pool), nil
	})
}

// ConsumerGroupPackage provides the consumer group with all registered consumers.
func ConsumerGroupPackage(i *do.Injector) {
	do.Provide(i, func(i *do.Injector) (*messaging.ConsumerGroup, error) {
		opts := do.MustInvoke[*Options](i)
		redisClient := do.MustInvoke[*RedisClient](i)
		logger := do.MustInvoke[*zap.Logger](i)
		store := do.MustInvoke[analytics.Store](i)

		subscriber, err := redisstream.NewSubscriber(
			redisstream.SubscriberConfig{
				Client:        redisClient.Client,
				ConsumerGroup: opts.ConsumerGroup,
				Consumer:      "consumer-1",
			},
			watermill.NewStdLogger(true, true),
		)
		if err != nil {
			return nil, err
		}

		group := messaging.NewConsumerGroup(subscriber, logger)

		// Register analytics consumers
		group.Add(messaging.NewConsumer(
			subscriber,
			opts.TopicURLCreated,
			store.SaveURLCreated,
			logger,
		))

		group.Add(messaging.NewConsumer(
			subscriber,
			opts.TopicURLAccessed,
			store.SaveURLAccessed,
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
		redisClient := do.MustInvoke[*RedisClient](i)
		urlStore := do.MustInvoke[shortener.Repository](i)
		rateLimitStore := do.MustInvoke[ratelimit.Store](i)
		publisherGroup := do.MustInvoke[*messaging.PublisherGroup](i)

		api := humachi.New(router, huma.DefaultConfig("URL Shortener", "1.0.0"))

		// Set up middleware
		api.UseMiddleware(middleware.RequestMeta(api))

		// Build rate limit policy from configuration
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, opts.RateLimitGlobalPerDay, 24*time.Hour).
			AddLimit(ratelimit.ScopeRead, opts.RateLimitReadPerMinute, time.Minute).
			AddLimit(ratelimit.ScopeWrite, opts.RateLimitWritePerMinute, time.Minute).
			AddLimit(ratelimit.ScopeWrite, opts.RateLimitWritePerHour, time.Hour).
			AddLimit(ratelimit.ScopeWrite, opts.RateLimitWritePerDay, 24*time.Hour).
			Build()

		limiter := ratelimit.NewPolicyLimiter(rateLimitStore, policy)
		resolver := ratelimit.NewOperationScopeResolver()
		api.UseMiddleware(middleware.PolicyRateLimiter(api, limiter, resolver, logger))

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
		healthHandler := health.NewHandler(health.NewRedisChecker(redisClient.Client))

		// Register routes
		handlers.RegisterRoutes(api, urlHandler)
		health.RegisterRoutes(api, healthHandler)

		return api, nil
	})
}

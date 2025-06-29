package ratelimiter

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RateLimiter struct {
	client   *redis.Client
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	limit    rate.Limit
	burst    int
	ttl      time.Duration
}

func NewRateLimiter(client *redis.Client, rps float64, burst int, ttl time.Duration) *RateLimiter {
	if client == nil {
		zap.L().Error("Redis client passed to NewRateLimiter is nil")
		panic("Redis client passed to NewRateLimiter is nil")
	}

	if ttl <= 0 {
		ttl = 5 * time.Minute
		zap.L().Warn("Invalid TTL provided to NewRateLimiter, defaulting", zap.Duration("default_ttl", ttl))
	}
	return &RateLimiter{
		client:   client,
		limiters: make(map[string]*rate.Limiter),
		limit:    rate.Limit(rps),
		burst:    burst,
		ttl:      ttl,
	}
}

func (rl *RateLimiter) GetLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	// Cek dulu apakah limiter sudah ada
	limiter, exists := rl.limiters[key]
	if !exists {
		newLimiterBurst := rl.burst
		ctx := context.Background()

		val, err := rl.client.Get(ctx, "ratelimit:"+key).Int()
		if err == nil && val > 0 {
			if val <= rl.burst {
				newLimiterBurst = val
			}
			zap.L().Debug(
				"Initializing limiter from Redis state",
				zap.String("key", key),
				zap.Int("redis_burst_val", val),
				zap.Int("initial_burst", newLimiterBurst),
			)
		} else if err != redis.Nil {
			zap.L().Error("Error getting rate limit state from Redis", zap.String("key", key), zap.Error(err))
		}

		limiter = rate.NewLimiter(rl.limit, newLimiterBurst)
		rl.limiters[key] = limiter

		time.AfterFunc(rl.ttl, func() {
			rl.mu.Lock()
			defer rl.mu.Unlock()
			zap.L().Debug("Removing limiter from memory due to TTL", zap.String("key", key))
			delete(rl.limiters, key)
		})
	}
	rl.mu.Unlock() 

	go func(lim *rate.Limiter, currentBurst int) {
		ctx := context.Background()
		err := rl.client.Set(ctx, "ratelimit:"+key, lim.Burst(), rl.ttl).Err()
		if err != nil {
			zap.L().Error("Error setting rate limit state to Redis", zap.String("key", key), zap.Error(err))
		}
	}(limiter, limiter.Burst())

	return limiter
}

func (rl *RateLimiter) RateLimitMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error { 
		key := c.IP()

		if key == "" {
			// Opsi:
			// 1. Tolak request (lebih aman)
			zap.L().Warn("Rate limiter cannot determine client IP address")
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message": "Access Forbidden: Cannot identify client.",
			})
			// 2. Izinkan (kurang aman, bypass rate limit)
			// return c.Next()
			// 3. Gunakan key default (berisiko bottleneck jika banyak IP tak dikenal)
			// key = "unknown_ip"
			// zaplog.Log.Debug("Rate limiter using default key for unknown IP")
		}

		limiter := rl.GetLimiter(key)

		if !limiter.Allow() {
			zap.L().Warn("Rate limit exceeded", zap.String("ip", key))

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"message": "Too many requests, please try again later.",
			})
		}

		return c.Next()
	}
}

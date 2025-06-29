package redisdb

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/fazamuttaqien/multifinance/config"
)

func NewRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.REDIS_ADDRESS,
		Password:     cfg.REDIS_PASSWORD,
		DB:           0,
		PoolSize:     10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		MaxRetries:   3,
		MinIdleConns: 2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		zap.L().Fatal(
			"Failed to ping Redis",
			zap.Error(err),
		)
		return nil, err
	}
	zap.L().Info("Redis terhubung! Response: " + pong)

	return client, nil
}

func MonitorRedis(cfg *config.Config) *redis.Client {
	var client *redis.Client
	var err error

	for {
		client, err = NewRedis(cfg)
		if err != nil {
			zap.L().Error(
				"Failed to connect to Redis, retrying in 5 seconds...",
				zap.Error(err),
			)
			time.Sleep(5 * time.Second)
		} else {
			zap.L().Info("Successfully connected to Redis")
			break
		}
	}

	return client
}

func WatchConnectionRedis(client **redis.Client, cfg *config.Config) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := (*client).Ping(ctx).Err()
		cancel()

		// If the ping fails, do a reconnect
		if err != nil {
			zap.L().Info("Failed to ping Redis, reconnecting...")

			// Disconnect the client first
			(*client).Close()

			// Trying to reconnect
			*client = MonitorRedis(cfg)
		}
	}
}
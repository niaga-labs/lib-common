package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// ConnectRedis establishes a connection to Redis
func ConnectRedis(config RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	logger.Info("Successfully connected to Redis",
		zap.String("host", config.Host),
		zap.Int("db", config.DB),
	)

	return client, nil
}

// HealthCheckRedis verifies the Redis connection is alive
func HealthCheckRedis(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return client.Ping(ctx).Err()
}

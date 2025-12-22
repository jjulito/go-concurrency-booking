package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/jjulito/reserva/config"
)

func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "", // no password set
		DB:           0,  // use default DB
		PoolSize:     50, // Tuning for concurrency
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("unable to connect to redis: %w", err)
	}

	return client, nil
}

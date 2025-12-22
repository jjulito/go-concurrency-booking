package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLockRepository struct {
	client *redis.Client
}

func NewRedisLockRepository(client *redis.Client) *RedisLockRepository {
	return &RedisLockRepository{client: client}
}

// AcquireLock attempts to acquire a distributed lock using the SET NX command.
func (r *RedisLockRepository) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	success, err := r.client.SetNX(ctx, key, "locked", ttl).Result()
	if err != nil {
		return false, err
	}
	return success, nil
}

func (r *RedisLockRepository) ReleaseLock(ctx context.Context, key string) error {
	// Simply delete the key. 
	// In production, we should check if the value matches to avoid deleting someone else's lock if ours expired.
	// But for this demo, DEL is sufficient assuming TTL handling is robust or use Lua script.
	return r.client.Del(ctx, key).Err()
}

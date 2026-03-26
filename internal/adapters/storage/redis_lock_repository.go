package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// releaseLockScript atomically deletes a Redis key only if the stored value
// matches the provided token, preventing a process from releasing a lock it no
// longer owns (e.g., after TTL expiry and re-acquisition by another process).
var releaseLockScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

type RedisLockRepository struct {
	client *redis.Client
}

func NewRedisLockRepository(client *redis.Client) *RedisLockRepository {
	return &RedisLockRepository{client: client}
}

// AcquireLock attempts to acquire a distributed lock using SET NX.
// Returns (acquired, token, error). The token must be passed to ReleaseLock
// to ensure only the owner can release the lock.
func (r *RedisLockRepository) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, string, error) {
	token := uuid.New().String()
	success, err := r.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, "", err
	}
	if !success {
		return false, "", nil
	}
	return true, token, nil
}

// ReleaseLock releases the lock only if the stored token matches.
// Uses a Lua script to make the check-and-delete atomic, preventing
// a process from accidentally deleting another process's lock after TTL expiry.
func (r *RedisLockRepository) ReleaseLock(ctx context.Context, key, token string) error {
	return releaseLockScript.Run(ctx, r.client, []string{key}, token).Err()
}

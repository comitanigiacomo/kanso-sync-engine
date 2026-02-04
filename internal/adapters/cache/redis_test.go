package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestRedisClient_Integration(t *testing.T) {
	_ = godotenv.Load("../../../.env")

	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")
	pass := getEnv("REDIS_PASSWORD", "secret_redis_pass_local")

	rdb, err := NewRedisClient(host, port, pass, 1)

	if err != nil {
		t.Skipf("Skipping Redis integration test: %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()

	require.NoError(t, rdb.FlushDB(ctx).Err(), "Failed to flush test DB")

	t.Run("Connection Ping", func(t *testing.T) {
		pong, err := rdb.Ping(ctx).Result()
		assert.NoError(t, err)
		assert.Equal(t, "PONG", pong)
	})

	t.Run("Set and Get Value", func(t *testing.T) {
		key := "test_key_60k"
		value := "hello redis"

		err := rdb.Set(ctx, key, value, 1*time.Minute).Err()
		require.NoError(t, err)

		val, err := rdb.Get(ctx, key).Result()
		assert.NoError(t, err)
		assert.Equal(t, value, val)

		rdb.Del(ctx, key)
	})

	t.Run("Expire Check", func(t *testing.T) {
		key := "test_expire"
		err := rdb.Set(ctx, key, "expire_me", 1*time.Second).Err()
		require.NoError(t, err)

		time.Sleep(1100 * time.Millisecond)

		_, err = rdb.Get(ctx, key).Result()

		assert.Error(t, err)
		assert.ErrorIs(t, err, redis.Nil, "Errors need to be of type 'redis.Nil'")
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		concurrency := 20
		done := make(chan bool)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				key := fmt.Sprintf("concurrent_key_%d", id)
				err := rdb.Set(ctx, key, "val", 10*time.Second).Err()
				assert.NoError(t, err)

				_, err = rdb.Get(ctx, key).Result()
				assert.NoError(t, err)

				done <- true
			}(i)
		}

		for i := 0; i < concurrency; i++ {
			<-done
		}
	})
}

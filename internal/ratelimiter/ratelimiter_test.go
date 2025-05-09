package ratelimiter

import (
	"context"
	"os"
	"testing"
	"time"

	"load-balancer/internal/logger"

	"github.com/redis/go-redis/v9"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2, 1, nil, "")
	clientID := "192.168.1.1"

	// Первый запрос
	start := time.Now()
	if !rl.Allow(clientID) {
		t.Error("Expected first request to be allowed")
	}
	// Проверяем количество токенов
	bucket := rl.buckets[clientID]
	if bucket.tokens != 1 {
		t.Errorf("Expected 1 token after first request, got %v", bucket.tokens)
	}

	// Второй запрос
	if !rl.Allow(clientID) {
		t.Error("Expected second request to be allowed")
	}
	if bucket.tokens != 0 {
		t.Errorf("Expected 0 tokens after second request, got %v", bucket.tokens)
	}

	// Третий запрос
	if rl.Allow(clientID) {
		t.Error("Expected third request to be denied")
	}

	// Ждем, пока не накопится хотя бы 1 токен
	for i := 0; i < 20; i++ {
		bucket.mu.Lock()
		elapsed := time.Since(bucket.lastRefill).Seconds()
		newTokens := elapsed * bucket.rate
		bucket.tokens = min(bucket.capacity, bucket.tokens+newTokens)
		bucket.lastRefill = time.Now()
		if bucket.tokens >= 1 {
			bucket.mu.Unlock()
			break
		}
		bucket.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	// Запрос после пополнения
	if !rl.Allow(clientID) {
		t.Error("Expected request to be allowed after refill")
	}
	// Проверяем количество токенов после вызова Allow
	if bucket.tokens < 0 || bucket.tokens > 0.5 {
		t.Errorf("Expected ~0 tokens after refill and consumption, got %v", bucket.tokens)
	}
	elapsed := time.Since(start).Seconds()
	t.Logf("Bucket state after refill: tokens=%v, lastRefill=%v, elapsed=%v seconds", bucket.tokens, bucket.lastRefill, elapsed)
}

func TestRateLimiter_UpdateClient(t *testing.T) {
	rl := NewRateLimiter(2, 1, nil, "")
	clientID := "192.168.1.1"

	rl.UpdateClient(clientID, 5, 2)
	if !rl.Allow(clientID) {
		t.Error("Expected request to be allowed after update")
	}

	// Проверяем обновление бакета
	bucket := rl.buckets[clientID]
	if bucket.capacity != 5 || bucket.rate != 2 {
		t.Errorf("Expected capacity=5, rate=2, got capacity=%v, rate=%v", bucket.capacity, bucket.rate)
	}
}

func TestRateLimiter_Redis(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		t.Skipf("Redis is not available, skipping test: %v", err)
	}
	defer redisClient.Close()

	rl := NewRateLimiter(2, 1, nil, "localhost:6379")
	rl.SetSyncRedis(true) // Включаем синхронное сохранение
	clientID := "192.168.1.1"

	if !rl.Allow(clientID) {
		t.Error("Expected first request to be allowed")
	}

	vals, err := redisClient.HGetAll(ctx, "ratelimit:"+clientID).Result()
	if err != nil {
		t.Fatalf("Failed to get data from Redis: %v", err)
	}
	t.Logf("Redis data: %v", vals)
	if vals["tokens"] != "1" {
		t.Errorf("Expected 1 token in Redis, got %q", vals["tokens"])
	}
}

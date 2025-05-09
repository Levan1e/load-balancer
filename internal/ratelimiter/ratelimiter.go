package ratelimiter

import (
	"context"
	"sync"
	"time"

	"load-balancer/internal/logger"
	"load-balancer/internal/models"

	"github.com/redis/go-redis/v9"
)

// RateLimiterInterface определяет методы для управления ограничением скорости запросов.
type RateLimiterInterface interface {
	Allow(clientID string) bool
	Update(capacity, rate float64)
	UpdateClient(clientID string, capacity, rate float64)
}

// RateLimiter управляет ограничением скорости запросов на основе токен-бакета.
type RateLimiter struct {
	buckets         map[string]*TokenBucket
	defaultCapacity float64
	defaultRate     float64
	clientConfigs   []models.ClientConfig
	redisClient     *redis.Client
	mu              sync.Mutex
	syncRedis       bool // Флаг для синхронного сохранения в тестах
}

// TokenBucket представляет токен-бакет для клиента.
type TokenBucket struct {
	tokens     float64
	lastRefill time.Time
	capacity   float64
	rate       float64
	mu         sync.Mutex
}

// NewRateLimiter создает новый RateLimiter с указанными параметрами.
func NewRateLimiter(capacity, rate float64, clientConfigs []models.ClientConfig, redisAddr string) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*TokenBucket),
		defaultCapacity: capacity,
		defaultRate:     rate,
		clientConfigs:   clientConfigs,
	}
	if redisAddr != "" {
		rl.redisClient = redis.NewClient(&redis.Options{Addr: redisAddr})
		// Проверяем подключение к Redis
		if err := rl.redisClient.Ping(context.Background()).Err(); err != nil {
			logger.ErrorKV("Failed to connect to Redis", "addr", redisAddr, "error", err)
		}
	}
	logger.InfoKV("Initializing RateLimiter", "default_capacity", capacity, "default_rate", rate)
	return rl
}

// SetSyncRedis включает синхронное сохранение в Redis для тестов.
func (rl *RateLimiter) SetSyncRedis(sync bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.syncRedis = sync
}

// Allow проверяет, разрешен ли запрос для указанного клиента.
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	bucket, exists := rl.buckets[clientID]
	if !exists {
		capacity, rate := rl.defaultCapacity, rl.defaultRate
		for _, cfg := range rl.clientConfigs {
			if cfg.ClientID == clientID {
				logger.InfoKV("Using client-specific rate limit", "clientID", clientID, "capacity", cfg.Capacity)
				capacity, rate = float64(cfg.Capacity), cfg.Rate
				break
			}
		}
		bucket = &TokenBucket{
			tokens:     capacity,
			lastRefill: time.Now(),
			capacity:   capacity,
			rate:       rate,
		}
		rl.buckets[clientID] = bucket
		logger.InfoKV("Creating new bucket", "clientID", clientID, "capacity", capacity)
	}
	rl.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Пополняем токены
	elapsed := time.Since(bucket.lastRefill).Seconds()
	newTokens := elapsed * bucket.rate
	bucket.tokens = min(bucket.capacity, bucket.tokens+newTokens)
	bucket.lastRefill = time.Now()
	logger.DebugKV("Refilled tokens", "clientID", clientID, "tokens", bucket.tokens, "elapsed", elapsed, "newTokens", newTokens)

	// Проверяем доступность токена
	if bucket.tokens < 1 {
		logger.WarnKV("Rate limit exceeded", "clientID", clientID, "tokens", bucket.tokens)
		return false
	}

	bucket.tokens--
	logger.InfoKV("Token consumed", "clientID", clientID, "remaining_tokens", bucket.tokens)

	// Сохраняем в Redis
	if rl.redisClient != nil {
		saveToRedis := func() {
			ctx := context.Background()
			err := rl.redisClient.HSet(ctx, "ratelimit:"+clientID, map[string]interface{}{
				"tokens":      bucket.tokens,
				"last_refill": bucket.lastRefill.UnixNano(),
				"capacity":    bucket.capacity,
				"rate":        bucket.rate,
			}).Err()
			if err != nil {
				logger.ErrorKV("Failed to save to Redis", "clientID", clientID, "error", err)
			} else {
				logger.DebugKV("Successfully saved to Redis", "clientID", clientID, "tokens", bucket.tokens)
			}
		}

		rl.mu.Lock()
		syncRedis := rl.syncRedis
		rl.mu.Unlock()

		if syncRedis {
			saveToRedis()
		} else {
			go saveToRedis()
		}
	}
	return true
}

// Update обновляет глобальные параметры rate-limiting.
func (rl *RateLimiter) Update(capacity, rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.defaultCapacity = capacity
	rl.defaultRate = rate
	logger.InfoKV("Updated rate limiter", "capacity", capacity, "rate", rate)
}

// UpdateClient обновляет параметры rate-limiting для конкретного клиента.
func (rl *RateLimiter) UpdateClient(clientID string, capacity, rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, exists := rl.buckets[clientID]; exists {
		bucket.mu.Lock()
		bucket.capacity = capacity
		bucket.rate = rate
		bucket.tokens = min(bucket.tokens, capacity)
		bucket.mu.Unlock()
	}

	for i, cfg := range rl.clientConfigs {
		if cfg.ClientID == clientID {
			rl.clientConfigs[i].Capacity = int(capacity)
			rl.clientConfigs[i].Rate = rate
			logger.InfoKV("Updated client rate limit", "clientID", clientID, "capacity", capacity)
			return
		}
	}
	rl.clientConfigs = append(rl.clientConfigs, models.ClientConfig{
		ClientID: clientID,
		Capacity: int(capacity),
		Rate:     rate,
	})
	logger.InfoKV("Updated client rate limit", "clientID", clientID, "capacity", capacity)
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

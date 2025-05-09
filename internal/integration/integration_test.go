package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"load-balancer/internal/api"
	"load-balancer/internal/health"
	"load-balancer/internal/logger"
	"load-balancer/internal/models"
)

func TestIntegration(t *testing.T) {
	logger.Init()

	// Setup test backends
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Backend 1")
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Backend 2")
	}))
	defer backend2.Close()

	// Create config
	cfg := &models.Config{
		Port: "8087",
		Backends: []*models.Backend{
			{URL: backend1.URL, Healthy: true},
			{URL: backend2.URL, Healthy: true},
		},
		HealthCheckPath:     "/health",
		HealthCheckInterval: 5 * time.Second,
		RateLimit: models.RateLimitConfig{
			Capacity: 10,
			Rate:     1,
		},
	}

	// Initialize server
	healthChecker := health.NewHealthChecker()
	server := api.NewServer(cfg.Backends, healthChecker, cfg.RateLimit.Capacity, cfg.RateLimit.Rate, nil, "", "configs/config.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start health checker
	healthChecker.Start(ctx, cfg, cfg.HealthCheckInterval)

	// Start server in a goroutine
	go func() {
		if err := server.Start("8087"); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server failed: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	t.Run("Round-robin distribution", func(t *testing.T) {
		client := &http.Client{}
		for i := 0; i < 2; i++ {
			resp, err := client.Get("http://localhost:8087/")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}
	})

	t.Run("Rate limiting", func(t *testing.T) {
		client := &http.Client{}
		for i := 0; i < 11; i++ {
			resp, err := client.Get("http://localhost:8087/")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()
			if i == 10 && resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("Expected status 429 on 11th request, got %d", resp.StatusCode)
			}
		}
	})

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

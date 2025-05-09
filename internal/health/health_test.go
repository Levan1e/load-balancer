package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"load-balancer/internal/logger"
	"load-balancer/internal/models"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestHealthChecker_Start(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer backendServer.Close()

	cfg := &models.Config{
		Backends:        []*models.Backend{{URL: backendServer.URL, Healthy: false}},
		HealthCheckPath: "/health",
	}
	healthChecker := NewHealthChecker()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthChecker.Start(ctx, cfg, 1*time.Second)
	healthChecker.WaitFirstCheck()

	if !cfg.Backends[0].Healthy {
		t.Error("Expected backend to be healthy")
	}
}

func TestHealthChecker_UnhealthyBackend(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backendServer.Close()

	cfg := &models.Config{
		Backends:        []*models.Backend{{URL: backendServer.URL, Healthy: true}},
		HealthCheckPath: "/health",
	}
	healthChecker := NewHealthChecker()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthChecker.Start(ctx, cfg, 1*time.Second)
	healthChecker.WaitFirstCheck()

	if cfg.Backends[0].Healthy {
		t.Error("Expected backend to be unhealthy")
	}
}

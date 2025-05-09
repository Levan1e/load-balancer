package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"load-balancer/internal/logger"
	"load-balancer/internal/models"
	"load-balancer/pkg/httpclient"
)

// HealthChecker performs periodic health checks on backends.
type HealthChecker struct {
	client     *http.Client
	firstCheck chan struct{} // Signal for completion of the first check (for tests)
	once       sync.Once     // Ensures single initialization of firstCheck
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		client:     httpclient.NewClient(5 * time.Second),
		firstCheck: make(chan struct{}),
	}
}

// Client returns the HTTP client used by the HealthChecker.
func (hc *HealthChecker) Client() *http.Client {
	return hc.client
}

// Start begins periodic health checks for the given backends.
func (hc *HealthChecker) Start(ctx context.Context, cfg *models.Config, interval time.Duration) {
	if interval <= 0 {
		logger.FatalKV("Health check interval must be positive", "interval", interval)
	}
	logger.InfoKV("Starting health checker", "interval", interval)
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Info("Health checker stopped")
				return
			case <-ticker.C:
				for _, backend := range cfg.Backends {
					req, err := http.NewRequestWithContext(ctx, "GET", backend.URL+cfg.HealthCheckPath, nil)
					if err != nil {
						logger.ErrorKV("Failed to create health check request", "url", backend.URL, "error", err)
						backend.Healthy = false
						backend.LastChecked = time.Now()
						logger.WarnKV("Backend is unhealthy", "url", backend.URL, "error", err)
						continue
					}
					resp, err := hc.client.Do(req)
					backend.LastChecked = time.Now()
					if err != nil {
						logger.WarnKV("Health check failed", "url", backend.URL, "error", err)
						backend.Healthy = false
						logger.WarnKV("Backend is unhealthy", "url", backend.URL, "error", err)
						continue
					}
					defer resp.Body.Close()
					backend.Healthy = resp.StatusCode == http.StatusOK
					if backend.Healthy {
						if !backend.LoggedHealthy {
							logger.InfoKV("Backend is healthy", "url", backend.URL)
							backend.LoggedHealthy = true
						}
					} else {
						logger.WarnKV("Backend is unhealthy", "url", backend.URL, "status_code", resp.StatusCode, "status_text", resp.Status)
						backend.LoggedHealthy = false
					}
				}
				hc.once.Do(func() {
					close(hc.firstCheck)
				})
			}
		}
	}()
}

// WaitFirstCheck waits for the first health check to complete (for testing purposes).
func (hc *HealthChecker) WaitFirstCheck() {
	<-hc.firstCheck
}

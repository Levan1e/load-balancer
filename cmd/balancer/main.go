package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"load-balancer/internal/api"
	"load-balancer/internal/config"
	"load-balancer/internal/health"
	"load-balancer/internal/logger"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.json")
	if err != nil {
		logger.ErrorKV("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize health checker
	healthChecker := health.NewHealthChecker()

	// Create server
	server := api.NewServer(
		cfg.Backends,
		healthChecker,
		cfg.RateLimit.Capacity,
		cfg.RateLimit.Rate,
		cfg.ClientConfigs,
		"redis:6379",          // Redis address
		"configs/config.json", // Path to config.json
	)

	// Start health checker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	healthChecker.Start(ctx, cfg, cfg.HealthCheckInterval)

	// Start server
	go func() {
		if err := server.Start(cfg.Port); err != nil && err != http.ErrServerClosed {
			logger.ErrorKV("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		logger.ErrorKV("Server shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("Server stopped")
}

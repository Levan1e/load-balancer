package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"load-balancer/internal/domain"
	"load-balancer/internal/logger"
	"load-balancer/internal/models"
)

// LoadConfig loads configuration from a JSON file.
func LoadConfig(path string) (*models.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logger.ErrorKV("Config file does not exist", "path", path)
		return nil, fmt.Errorf("config file %s does not exist", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logger.ErrorKV("Failed to read config file", "path", path, "error", err)
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	logger.InfoKV("Config file read successfully", "path", path)

	var cfg struct {
		Port                string                 `json:"port"`
		Backends            []string               `json:"backends"`
		HealthCheckPath     string                 `json:"health_check_path"`
		HealthCheckInterval string                 `json:"health_check_interval"`
		RateLimit           models.RateLimitConfig `json:"rate_limit"`
		ClientConfigs       []models.ClientConfig  `json:"client_configs"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.ErrorKV("Failed to unmarshal config", "error", err)
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Parse health_check_interval
	var healthCheckInterval time.Duration
	if cfg.HealthCheckInterval != "" {
		var err error
		healthCheckInterval, err = time.ParseDuration(cfg.HealthCheckInterval)
		if err != nil {
			logger.ErrorKV("Invalid health_check_interval", "value", cfg.HealthCheckInterval, "error", err)
			return nil, domain.ErrInvalidConfig
		}
	} else {
		healthCheckInterval = 5 * time.Second
		logger.InfoKV("Using default health check interval", "interval", "5s")
	}

	// Convert string backends to []*models.Backend
	backends := make([]*models.Backend, len(cfg.Backends))
	for i, url := range cfg.Backends {
		backends[i] = &models.Backend{
			URL:           url,
			Healthy:       true,
			LoggedHealthy: false,
		}
	}

	// Log environment variables for debugging
	backendsEnv := os.Getenv("BACKENDS")
	portEnv := os.Getenv("PORT")
	rateLimitCapacityEnv := os.Getenv("RATE_LIMIT_CAPACITY")
	rateLimitRateEnv := os.Getenv("RATE_LIMIT_RATE")
	logger.InfoKV("Checking environment variables", "BACKENDS", backendsEnv, "PORT", portEnv, "RATE_LIMIT_CAPACITY", rateLimitCapacityEnv, "RATE_LIMIT_RATE", rateLimitRateEnv)

	// Override backends from environment variable if set
	if backendsEnv != "" {
		backendURLs := strings.Split(backendsEnv, ",")
		backends = make([]*models.Backend, len(backendURLs))
		for i, url := range backendURLs {
			backends[i] = &models.Backend{
				URL:           strings.TrimSpace(url),
				Healthy:       true,
				LoggedHealthy: false,
			}
		}
		logger.InfoKV("Backends overridden from environment", "backends", backendURLs)
	}

	port := strings.TrimPrefix(cfg.Port, ":")
	if portEnv != "" {
		port = strings.TrimPrefix(portEnv, ":")
		logger.InfoKV("Port overridden from environment", "port", port)
	}

	// Override rate limit settings from environment variables if set
	rateLimit := cfg.RateLimit
	if rateLimitCapacityEnv != "" {
		if capacity, err := strconv.Atoi(rateLimitCapacityEnv); err == nil && capacity > 0 {
			rateLimit.Capacity = capacity
			logger.InfoKV("Rate limit capacity overridden from environment", "capacity", capacity)
		}
	}
	if rateLimitRateEnv != "" {
		if rate, err := strconv.ParseFloat(rateLimitRateEnv, 64); err == nil && rate > 0 {
			rateLimit.Rate = rate
			logger.InfoKV("Rate limit rate overridden from environment", "rate", rate)
		}
	}

	finalCfg := &models.Config{
		Port:                port,
		Backends:            backends,
		HealthCheckPath:     cfg.HealthCheckPath,
		HealthCheckInterval: healthCheckInterval,
		RateLimit:           rateLimit,
		ClientConfigs:       cfg.ClientConfigs,
	}

	// Validate configuration
	if finalCfg.Port == "" {
		logger.Error("Port is not specified in config")
		return nil, domain.ErrInvalidConfig
	}
	if len(finalCfg.Backends) == 0 {
		logger.Error("No backends specified in config")
		return nil, domain.ErrInvalidConfig
	}
	if finalCfg.HealthCheckPath == "" {
		finalCfg.HealthCheckPath = "/health"
		logger.InfoKV("Using default health check path", "path", "/health")
	}
	if finalCfg.HealthCheckInterval <= 0 {
		finalCfg.HealthCheckInterval = 5 * time.Second
		logger.InfoKV("Using default health check interval", "interval", "5s")
	}
	if finalCfg.RateLimit.Capacity <= 0 {
		logger.Error("Rate limit capacity must be positive")
		return nil, domain.ErrInvalidConfig
	}
	if finalCfg.RateLimit.Rate <= 0 {
		logger.Error("Rate limit rate must be positive")
		return nil, domain.ErrInvalidConfig
	}
	for i, client := range finalCfg.ClientConfigs {
		logger.DebugKV("Validating client config", "index", i, "client_id", client.ClientID, "capacity", client.Capacity, "rate", client.Rate)
		if client.ClientID == "" {
			logger.ErrorKV("Client ID is empty in client_configs", "index", i)
			return nil, domain.ErrInvalidConfig
		}
		if client.Capacity <= 0 {
			logger.ErrorKV("Client capacity must be positive", "client_id", client.ClientID, "index", i)
			return nil, domain.ErrInvalidConfig
		}
		if client.Rate <= 0 {
			logger.ErrorKV("Client rate must be positive", "client_id", client.ClientID, "index", i)
			return nil, domain.ErrInvalidConfig
		}
	}

	logger.InfoKV("Configuration loaded", "port", finalCfg.Port, "backends", len(finalCfg.Backends), "health_check_path", finalCfg.HealthCheckPath, "health_check_interval", finalCfg.HealthCheckInterval, "rate_limit_capacity", finalCfg.RateLimit.Capacity, "rate_limit_rate", finalCfg.RateLimit.Rate, "client_configs", len(finalCfg.ClientConfigs))
	return finalCfg, nil
}

// SaveConfig saves the configuration to a JSON file.
func SaveConfig(path string, cfg *models.Config) error {
	if cfg.Port == "" {
		cfg.Port = "8087"
		logger.InfoKV("Using default port for save", "port", cfg.Port)
	}
	if cfg.HealthCheckPath == "" {
		cfg.HealthCheckPath = "/health"
		logger.InfoKV("Using default health check path for save", "path", cfg.HealthCheckPath)
	}
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 5 * time.Second
		logger.InfoKV("Using default health check interval for save", "interval", cfg.HealthCheckInterval)
	}

	// Prepare config for serialization
	configData := struct {
		Port                string                 `json:"port"`
		Backends            []string               `json:"backends"`
		HealthCheckPath     string                 `json:"health_check_path"`
		HealthCheckInterval string                 `json:"health_check_interval"`
		RateLimit           models.RateLimitConfig `json:"rate_limit"`
		ClientConfigs       []models.ClientConfig  `json:"client_configs"`
	}{
		Port:                ":" + strings.TrimPrefix(cfg.Port, ":"),
		Backends:            make([]string, len(cfg.Backends)),
		HealthCheckPath:     cfg.HealthCheckPath,
		HealthCheckInterval: cfg.HealthCheckInterval.String(),
		RateLimit:           cfg.RateLimit,
		ClientConfigs:       cfg.ClientConfigs,
	}
	for i, backend := range cfg.Backends {
		configData.Backends[i] = backend.URL
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		logger.ErrorKV("Failed to marshal config", "error", err)
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		logger.ErrorKV("Failed to write config file", "path", path, "error", err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logger.InfoKV("Config file saved successfully", "path", path)
	return nil
}

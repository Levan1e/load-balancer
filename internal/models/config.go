package models

import "time"

// RateLimitConfig holds rate-limiting configuration.
type RateLimitConfig struct {
	Capacity int     `json:"capacity"`
	Rate     float64 `json:"rate"`
}

// ClientConfig holds client-specific rate-limiting configuration.
type ClientConfig struct {
	ClientID string  `json:"client_id" mapstructure:"client_id"`
	Capacity int     `json:"capacity" mapstructure:"capacity"`
	Rate     float64 `json:"rate" mapstructure:"rate"`
}

// Config holds the application configuration.
type Config struct {
	Port                string          `json:"port"`
	Backends            []*Backend      `json:"backends"`
	HealthCheckPath     string          `json:"health_check_path"`
	HealthCheckInterval time.Duration   `json:"health_check_interval"`
	RateLimit           RateLimitConfig `json:"rate_limit"`
	ClientConfigs       []ClientConfig  `json:"client_configs"`
}

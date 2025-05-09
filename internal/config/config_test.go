package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"load-balancer/internal/domain"
	"load-balancer/internal/logger"
	"load-balancer/internal/models"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestLoadConfig(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "test_config.json")

	configContent := `{
		"port": ":8087",
		"backends": ["http://localhost:8001", "http://localhost:8002"],
		"health_check_path": "/health",
		"health_check_interval": "5s",
		"rate_limit": {"capacity": 100, "rate": 10},
		"client_configs": [
			{"client_id": "192.168.1.1", "capacity": 50, "rate": 5}
		]
	}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		path          string
		env           map[string]string
		expectedError bool
		expected      *models.Config
	}{
		{
			name: "Valid config",
			path: configPath,
			expected: &models.Config{
				Port:                "8087",
				Backends:            []*models.Backend{{URL: "http://localhost:8001", Healthy: true}, {URL: "http://localhost:8002", Healthy: true}},
				HealthCheckPath:     "/health",
				HealthCheckInterval: 5 * time.Second,
				RateLimit:           models.RateLimitConfig{Capacity: 100, Rate: 10},
				ClientConfigs:       []models.ClientConfig{{ClientID: "192.168.1.1", Capacity: 50, Rate: 5}},
			},
		},
		{
			name:          "Non-existent file",
			path:          filepath.Join(configDir, "nonexistent.json"),
			expectedError: true,
		},
		{
			name: "Override with env",
			path: configPath,
			env: map[string]string{
				"BACKENDS":            "http://localhost:8003",
				"PORT":                ":8088",
				"RATE_LIMIT_CAPACITY": "200",
				"RATE_LIMIT_RATE":     "20",
			},
			expected: &models.Config{
				Port:                "8088",
				Backends:            []*models.Backend{{URL: "http://localhost:8003", Healthy: true}},
				HealthCheckPath:     "/health",
				HealthCheckInterval: 5 * time.Second,
				RateLimit:           models.RateLimitConfig{Capacity: 200, Rate: 20},
				ClientConfigs:       []models.ClientConfig{{ClientID: "192.168.1.1", Capacity: 50, Rate: 5}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			cfg, err := LoadConfig(tt.path)
			if tt.expectedError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if cfg.Port != tt.expected.Port {
				t.Errorf("Expected port %v, got %v", tt.expected.Port, cfg.Port)
			}
			if len(cfg.Backends) != len(tt.expected.Backends) {
				t.Errorf("Expected %d backends, got %d", len(tt.expected.Backends), len(cfg.Backends))
			} else {
				for i, b := range cfg.Backends {
					if b.URL != tt.expected.Backends[i].URL {
						t.Errorf("Expected backend URL %v, got %v", tt.expected.Backends[i].URL, b.URL)
					}
				}
			}
			if cfg.RateLimit.Capacity != tt.expected.RateLimit.Capacity {
				t.Errorf("Expected rate limit capacity %v, got %v", tt.expected.RateLimit.Capacity, cfg.RateLimit.Capacity)
			}
			if cfg.HealthCheckInterval != tt.expected.HealthCheckInterval {
				t.Errorf("Expected health check interval %v, got %v", tt.expected.HealthCheckInterval, cfg.HealthCheckInterval)
			}
		})
	}
}

func TestLoadConfig_InvalidConfig(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "invalid_config.json")

	configContent := `{
		"port": "",
		"backends": [],
		"health_check_interval": "invalid",
		"rate_limit": {"capacity": 0, "rate": 0}
	}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(configPath)
	if err != domain.ErrInvalidConfig {
		t.Errorf("Expected ErrInvalidConfig, got %v", err)
	}
}

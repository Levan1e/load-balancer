package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"load-balancer/internal/health"
	"load-balancer/internal/logger"
	"load-balancer/internal/models"
)

func TestServer_HandleRequest(t *testing.T) {
	logger.Init()
	healthChecker := health.NewHealthChecker()

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	}))
	defer backendServer.Close()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.json")

	server := NewServer(
		[]*models.Backend{{URL: backendServer.URL, Healthy: true}},
		healthChecker,
		10, 1,
		[]models.ClientConfig{{ClientID: "127.0.0.1", Capacity: 10, Rate: 1}},
		"", configPath,
	)

	t.Run("Successful request", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		server.handleRequest(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		if rr.Body.String() != "OK" {
			t.Errorf("Expected body 'OK', got %s", rr.Body.String())
		}
	})

	t.Run("Rate limit exceeded", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.2:12345"
		rr := httptest.NewRecorder()

		for i := 0; i < 10; i++ {
			server.handleRequest(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200 for request %d, got %d", i+1, rr.Code)
			}
			rr = httptest.NewRecorder()
		}

		server.handleRequest(rr, req)
		var errResp ErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		if rr.Code != http.StatusTooManyRequests || errResp.Message != "Rate limit exceeded" {
			t.Errorf("Expected status 429 and 'Rate limit exceeded', got %d and %v", rr.Code, errResp)
		}
	})

	t.Run("No healthy backends", func(t *testing.T) {
		server := NewServer(
			[]*models.Backend{{URL: backendServer.URL, Healthy: false}},
			healthChecker,
			10, 1,
			nil, "", configPath,
		)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		server.handleRequest(rr, req)

		var errResp ErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		if rr.Code != http.StatusServiceUnavailable || errResp.Message != "No healthy backends available" {
			t.Errorf("Expected status 503 and 'No healthy backends available', got %d and %v", rr.Code, errResp)
		}
	})
}

func TestServer_HandleBackends(t *testing.T) {
	logger.Init()
	healthChecker := health.NewHealthChecker()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.json")

	server := NewServer(
		[]*models.Backend{{URL: "http://localhost:8001", Healthy: true}},
		healthChecker,
		10, 1,
		nil, "", configPath,
	)

	t.Run("GET backends", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/backends", nil)
		rr := httptest.NewRecorder()
		server.handleBackends(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		var backends []*models.Backend
		if err := json.NewDecoder(rr.Body).Decode(&backends); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		if len(backends) != 1 || backends[0].URL != "http://localhost:8001" {
			t.Errorf("Expected 1 backend with URL 'http://localhost:8001', got %v", backends)
		}
	})

	t.Run("POST new backend", func(t *testing.T) {
		configsDir := filepath.Join(configDir, "configs")
		if err := os.MkdirAll(configsDir, 0755); err != nil {
			t.Fatalf("Failed to create configs directory: %v", err)
		}

		body := bytes.NewBufferString(`{"url": "http://localhost:8002"}`)
		req, _ := http.NewRequest("POST", "/api/backends", body)
		rr := httptest.NewRecorder()
		server.handleBackends(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}
	})

	t.Run("DELETE backend", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/backends?url=http://localhost:8001", nil)
		rr := httptest.NewRecorder()
		server.handleBackends(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}
	})
}

func TestServer_HandleRateLimit(t *testing.T) {
	logger.Init()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.json")

	server := NewServer(nil, health.NewHealthChecker(), 10, 1, nil, "", configPath)

	t.Run("PATCH rate limit", func(t *testing.T) {
		body := bytes.NewBufferString(`{"capacity": 20, "rate": 2}`)
		req, _ := http.NewRequest("PATCH", "/api/ratelimit", body)
		rr := httptest.NewRecorder()
		server.handleRateLimit(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}
	})
}

func TestServer_HandleClients(t *testing.T) {
	logger.Init()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.json")

	server := NewServer(nil, health.NewHealthChecker(), 10, 1, nil, "", configPath)

	t.Run("POST new client", func(t *testing.T) {
		body := bytes.NewBufferString(`{"client_id": "192.168.1.3", "capacity": 30, "rate": 3}`)
		req, _ := http.NewRequest("POST", "/api/clients", body)
		rr := httptest.NewRecorder()
		server.handleClients(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}
	})

	t.Run("DELETE client", func(t *testing.T) {
		server.rateLimiter.UpdateClient("192.168.1.3", 30, 3)
		req, _ := http.NewRequest("DELETE", "/api/clients?client_id=192.168.1.3", nil)
		rr := httptest.NewRecorder()
		server.handleClients(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rr.Code)
		}
	})
}

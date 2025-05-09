package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"load-balancer/docs"
	"load-balancer/internal/balancer"
	"load-balancer/internal/config"
	"load-balancer/internal/health"
	"load-balancer/internal/logger"
	"load-balancer/internal/models"
	"load-balancer/internal/proxy"
	"load-balancer/internal/ratelimiter"

	httpSwagger "github.com/swaggo/http-swagger"
)

// ErrorResponse represents a structured JSON error response.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server manages the HTTP server and request balancing.
type Server struct {
	cfg         *models.Config
	configPath  string // Path to config.json for saving changes
	health      *health.HealthChecker
	rateLimiter ratelimiter.RateLimiterInterface
	server      *http.Server
	mu          sync.RWMutex
	balancer    balancer.BalancerInterface
	proxy       *proxy.Proxy
}

// NewServer initializes a new server with backends, health checker, and rate-limiting parameters.
func NewServer(backends []*models.Backend, health *health.HealthChecker, rateLimitCapacity int, rateLimitRate float64, clientConfigs []models.ClientConfig, redisAddr, configPath string) *Server {
	cfg := &models.Config{
		Port:                ":8087",
		Backends:            backends,
		HealthCheckPath:     "/health",
		HealthCheckInterval: 5 * time.Second,
		RateLimit: models.RateLimitConfig{
			Capacity: rateLimitCapacity,
			Rate:     rateLimitRate,
		},
		ClientConfigs: clientConfigs,
	}
	rl := ratelimiter.NewRateLimiter(float64(rateLimitCapacity), rateLimitRate, clientConfigs, redisAddr)
	return &Server{
		cfg:         cfg,
		configPath:  configPath,
		health:      health,
		rateLimiter: rl,
		balancer:    balancer.NewBalancer(backends),
		proxy:       proxy.NewProxy(),
	}
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	docs.SwaggerInfo.Title = "Load Balancer API"
	docs.SwaggerInfo.Description = "API for managing load balancer backends and rate-limiting configurations."
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:8087"
	docs.SwaggerInfo.BasePath = "/api"
	docs.SwaggerInfo.Schemes = []string{"http"}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/api/backends", s.handleBackends)
	mux.HandleFunc("/api/ratelimit", s.handleRateLimit)
	mux.HandleFunc("/api/clients", s.handleClients)
	mux.Handle("/swagger/", httpSwagger.WrapHandler)
	return mux
}

// sendError sends a JSON error response with the specified code and message.
func (s *Server) sendError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	errResp := ErrorResponse{Code: code, Message: message}
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		logger.ErrorKV("Failed to encode error response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleRequest processes incoming requests with rate-limiting and forwarding to backends.
// @Summary Forward request to backend
// @Description Forwards an incoming HTTP request to a healthy backend using round-robin balancing.
// @Produce plain
// @Success 200 {string} string "Response from backend"
// @Failure 429 {object} ErrorResponse "Rate limit exceeded"
// @Failure 503 {object} ErrorResponse "No healthy backends available"
// @Failure 502 {object} ErrorResponse "Failed to forward request"
// @Router / [get]
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Requests to /api/* or /swagger/* are handled by other handlers, otherwise return 404
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/swagger/") {
		s.sendError(w, http.StatusNotFound, "API endpoint not found")
		return
	}

	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	logger.DebugKV("Processing request", "clientIP", clientIP, "method", r.Method)

	// Check rate-limiting
	if !s.rateLimiter.Allow(clientIP) {
		logger.WarnKV("Request rejected due to rate limit", "clientIP", clientIP)
		s.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}

	// Select the next healthy backend
	backend := s.balancer.NextBackend()
	if backend == nil {
		logger.Warn("No healthy backends available")
		s.sendError(w, http.StatusServiceUnavailable, "No healthy backends available")
		return
	}

	logger.InfoKV("Forwarding request", "method", r.Method, "url", r.URL.String(), "backend", backend.URL)
	if err := s.proxy.Forward(w, r, backend.URL); err != nil {
		logger.ErrorKV("Failed to forward request", "backend", backend.URL, "error", err)
		s.sendError(w, http.StatusBadGateway, fmt.Sprintf("Failed to forward request to %s", backend.URL))
	}
}

// handleBackends manages CRUD operations for backends.
// @Summary Manage backends
// @Description Get, add, or delete backend servers.
// @Tags Backends
// @Accept json
// @Produce json
// @Param url query string false "Backend URL (required for DELETE)"
// @Param body body object false "Backend URL (required for POST, e.g., {\"url\": \"http://backend3:80\"})"
// @Success 200 {array} models.Backend "List of backends (GET)"
// @Success 201 {string} string "Backend added (POST)"
// @Success 204 {string} string "Backend deleted (DELETE)"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 409 {object} ErrorResponse "Backend already exists"
// @Failure 404 {object} ErrorResponse "Backend not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /backends [get]
// @Router /backends [post]
// @Router /backends [delete]
func (s *Server) handleBackends(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		backends := s.cfg.Backends
		s.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(backends); err != nil {
			s.sendError(w, http.StatusInternalServerError, "Failed to encode backends")
			return
		}

	case http.MethodPost:
		var input struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			s.sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if input.URL == "" {
			s.sendError(w, http.StatusBadRequest, "Backend URL is required")
			return
		}

		// Validate URL
		if _, err := url.ParseRequestURI(input.URL); err != nil {
			s.sendError(w, http.StatusBadRequest, "Invalid backend URL")
			return
		}

		// Check if backend already exists
		s.mu.RLock()
		for _, b := range s.cfg.Backends {
			if b.URL == input.URL {
				s.mu.RUnlock()
				s.sendError(w, http.StatusConflict, fmt.Sprintf("Backend with URL %s already exists", input.URL))
				return
			}
		}
		s.mu.RUnlock()

		// Create new backend
		newBackend := &models.Backend{
			URL:           input.URL,
			Healthy:       false,
			LoggedHealthy: false,
		}

		// Perform immediate health check
		req, err := http.NewRequestWithContext(context.Background(), "GET", newBackend.URL+s.cfg.HealthCheckPath, nil)
		if err == nil {
			resp, err := s.health.Client().Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				newBackend.Healthy = true
				newBackend.LoggedHealthy = true
				logger.InfoKV("New backend is healthy", "url", newBackend.URL)
			} else {
				logger.WarnKV("New backend is unhealthy", "url", newBackend.URL, "error", err)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}

		// Generate unique index for HTML file
		s.mu.Lock()
		backendIndex := len(s.cfg.Backends) + 1
		s.cfg.Backends = append(s.cfg.Backends, newBackend)
		s.balancer = balancer.NewBalancer(s.cfg.Backends)
		s.mu.Unlock()

		// Create configs directory if it doesn't exist
		configsDir := filepath.Join(filepath.Dir(s.configPath), "configs")
		if err := os.MkdirAll(configsDir, 0755); err != nil {
			logger.ErrorKV("Failed to create configs directory", "error", err)
			s.sendError(w, http.StatusInternalServerError, "Failed to create configs directory")
			return
		}

		// Create HTML file for the backend
		htmlContent := fmt.Sprintf(`<!DOCTYPE html><html><head><title>Welcome to Nginx!</title></head><body><h1>Hello from Nginx Backend %d!</h1></body></html>`, backendIndex)
		htmlFilePath := filepath.Join(configsDir, fmt.Sprintf("index-backend%d.html", backendIndex))
		if err := os.WriteFile(htmlFilePath, []byte(htmlContent), 0644); err != nil {
			logger.ErrorKV("Failed to create HTML file", "path", htmlFilePath, "error", err)
			s.sendError(w, http.StatusInternalServerError, "Failed to create HTML file for backend")
			return
		}
		logger.InfoKV("Created HTML file for backend", "url", newBackend.URL, "path", htmlFilePath)

		// Save updated configuration to config.json
		if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
			logger.ErrorKV("Failed to save config", "error", err)
			s.sendError(w, http.StatusInternalServerError, "Failed to save configuration")
			return
		}

		logger.InfoKV("Successfully added new backend", "url", newBackend.URL, "index", backendIndex)
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		// Extract backend URL from query parameter
		backendURL := r.URL.Query().Get("url")
		if backendURL == "" {
			s.sendError(w, http.StatusBadRequest, "Backend URL is required as query parameter 'url'")
			return
		}

		// Check if backend exists
		s.mu.Lock()
		var backendIndex = -1
		for i, b := range s.cfg.Backends {
			if b.URL == backendURL {
				backendIndex = i
				break
			}
		}
		if backendIndex == -1 {
			s.mu.Unlock()
			s.sendError(w, http.StatusNotFound, fmt.Sprintf("Backend with URL %s not found", backendURL))
			return
		}

		// Remove backend from configuration
		s.cfg.Backends = append(s.cfg.Backends[:backendIndex], s.cfg.Backends[backendIndex+1:]...)
		s.balancer = balancer.NewBalancer(s.cfg.Backends)
		s.mu.Unlock()

		// Save updated configuration to config.json
		if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
			logger.ErrorKV("Failed to save config", "error", err)
			s.sendError(w, http.StatusInternalServerError, "Failed to save configuration")
			return
		}

		logger.InfoKV("Successfully deleted backend", "url", backendURL)
		w.WriteHeader(http.StatusNoContent)

	default:
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleRateLimit updates rate-limiting parameters.
// @Summary Update global rate limit
// @Description Update the global rate-limiting parameters (capacity and rate).
// @Tags RateLimit
// @Accept json
// @Produce json
// @Param body body object true "Rate limit parameters (e.g., {\"capacity\": 100, \"rate\": 10})"
// @Success 204 {string} string "Rate limit updated"
// @Failure 400 {object} ErrorResponse "Invalid request body"
// @Failure 500 {object} ErrorResponse "Failed to save configuration"
// @Router /ratelimit [patch]
func (s *Server) handleRateLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var params struct {
		Capacity float64 `json:"capacity"`
		Rate     float64 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate parameters
	if params.Capacity <= 0 {
		s.sendError(w, http.StatusBadRequest, "Capacity must be positive")
		return
	}
	if params.Rate <= 0 {
		s.sendError(w, http.StatusBadRequest, "Rate must be positive")
		return
	}

	// Update rate limiter
	s.mu.Lock()
	s.rateLimiter.Update(params.Capacity, params.Rate)
	s.cfg.RateLimit.Capacity = int(params.Capacity)
	s.cfg.RateLimit.Rate = params.Rate
	s.mu.Unlock()

	// Save updated configuration to config.json
	if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
		logger.ErrorKV("Failed to save config", "error", err)
		s.sendError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}

	logger.InfoKV("Successfully updated global rate limit", "capacity", params.Capacity, "rate", params.Rate)
	w.WriteHeader(http.StatusNoContent)
}

// handleClients manages CRUD operations for client rate-limiting configurations.
// @Summary Manage client rate limits
// @Description Get, add, or delete client-specific rate-limiting configurations.
// @Tags Clients
// @Accept json
// @Produce json
// @Param client_id query string false "Client ID (required for DELETE)"
// @Param body body models.ClientConfig false "Client configuration (required for POST, e.g., {\"client_id\": \"user1\", \"capacity\": 100, \"rate\": 10})"
// @Success 200 {array} models.ClientConfig "List of client configurations (GET)"
// @Success 201 {string} string "Client added (POST)"
// @Success 204 {string} string "Client deleted (DELETE)"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 409 {object} ErrorResponse "Client already exists"
// @Failure 404 {object} ErrorResponse "Client not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /clients [get]
// @Router /clients [post]
// @Router /clients [delete]
func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		clients := s.cfg.ClientConfigs
		s.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(clients); err != nil {
			s.sendError(w, http.StatusInternalServerError, "Failed to encode clients")
			return
		}

	case http.MethodPost:
		var client models.ClientConfig
		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			s.sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if client.ClientID == "" {
			logger.ErrorKV("Client ID is empty", "client_id", client.ClientID)
			s.sendError(w, http.StatusBadRequest, "Client ID is required")
			return
		}
		if client.Capacity <= 0 {
			logger.ErrorKV("Invalid client capacity", "client_id", client.ClientID, "capacity", client.Capacity)
			s.sendError(w, http.StatusBadRequest, "Capacity must be positive")
			return
		}
		if client.Rate <= 0 {
			logger.ErrorKV("Invalid client rate", "client_id", client.ClientID, "rate", client.Rate)
			s.sendError(w, http.StatusBadRequest, "Rate must be positive")
			return
		}

		// Check if client already exists
		s.mu.RLock()
		for _, c := range s.cfg.ClientConfigs {
			if c.ClientID == client.ClientID {
				s.mu.RUnlock()
				s.sendError(w, http.StatusConflict, fmt.Sprintf("Client with ID %s already exists", client.ClientID))
				return
			}
		}
		s.mu.RUnlock()

		// Add client to configuration
		s.mu.Lock()
		s.cfg.ClientConfigs = append(s.cfg.ClientConfigs, client)
		s.rateLimiter.UpdateClient(client.ClientID, float64(client.Capacity), client.Rate)
		s.mu.Unlock()

		// Save updated configuration to config.json
		if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
			logger.ErrorKV("Failed to save config", "error", err)
			s.sendError(w, http.StatusInternalServerError, "Failed to save configuration")
			return
		}

		logger.InfoKV("Successfully added new client", "client_id", client.ClientID, "capacity", client.Capacity, "rate", client.Rate)
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		// Extract client ID from query parameter
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			s.sendError(w, http.StatusBadRequest, "Client ID is required as query parameter 'client_id'")
			return
		}

		s.mu.Lock()
		for i, c := range s.cfg.ClientConfigs {
			if c.ClientID == clientID {
				s.cfg.ClientConfigs = append(s.cfg.ClientConfigs[:i], s.cfg.ClientConfigs[i+1:]...)
				s.rateLimiter.UpdateClient(clientID, float64(s.cfg.RateLimit.Capacity), s.cfg.RateLimit.Rate)
				s.mu.Unlock()

				// Save updated configuration to config.json
				if err := config.SaveConfig(s.configPath, s.cfg); err != nil {
					logger.ErrorKV("Failed to save config", "error", err)
					s.sendError(w, http.StatusInternalServerError, "Failed to save configuration")
					return
				}

				logger.InfoKV("Successfully deleted client", "client_id", clientID)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		s.mu.Unlock()
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Client with ID %s not found", clientID))

	default:
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// Start launches the server on the specified port.
func (s *Server) Start(port string) error {
	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: s.Handler(),
	}
	logger.InfoKV("Starting server", "port", port)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	logger.Info("Shutting down server")
	return s.server.Shutdown(ctx)
}

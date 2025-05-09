package balancer

import (
	"os"
	"sync"
	"testing"

	"load-balancer/internal/logger"
	"load-balancer/internal/models"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestBalancer_NextBackend(t *testing.T) {
	backends := []*models.Backend{
		{URL: "http://localhost:8001", Healthy: true},
		{URL: "http://localhost:8002", Healthy: false},
		{URL: "http://localhost:8003", Healthy: true},
	}
	balancer := NewBalancer(backends)

	tests := []struct {
		name          string
		expectedURLs  []string
		expectedNil   bool
		backendStates []bool
	}{
		{
			name:         "Select healthy backends",
			expectedURLs: []string{"http://localhost:8001", "http://localhost:8003", "http://localhost:8001"},
		},
		{
			name:          "No healthy backends",
			expectedNil:   true,
			backendStates: []bool{false, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balancer.ResetCurrent()
			if len(tt.backendStates) > 0 {
				for i, state := range tt.backendStates {
					backends[i].Healthy = state
				}
			}
			if tt.expectedNil {
				backend := balancer.NextBackend()
				if backend != nil {
					t.Errorf("Expected nil backend, got %v", backend.URL)
				}
				return
			}
			for i, expectedURL := range tt.expectedURLs {
				backend := balancer.NextBackend()
				if backend == nil || backend.URL != expectedURL {
					t.Errorf("Call %d: Expected backend %v, got %v", i+1, expectedURL, backend)
				}
			}
		})
	}
}

func TestBalancer_ConcurrentAccess(t *testing.T) {
	backends := []*models.Backend{
		{URL: "http://localhost:8001", Healthy: true},
		{URL: "http://localhost:8002", Healthy: true},
	}
	balancer := NewBalancer(backends)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			backend := balancer.NextBackend()
			if backend == nil {
				t.Error("Expected non-nil backend")
			}
		}()
	}
	wg.Wait()
}

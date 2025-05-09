package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"load-balancer/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestProxy_Forward(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	}))
	defer backendServer.Close()

	proxy := NewProxy()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	err = proxy.Forward(rr, req, backendServer.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %v", rr.Code)
	}
	if rr.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %v", rr.Body.String())
	}
}

func TestProxy_Forward_UnavailableBackend(t *testing.T) {
	proxy := NewProxy()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	err = proxy.Forward(rr, req, "http://nonexistent:9999")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if rr.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %v", rr.Code)
	}
}

package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"

	"load-balancer/internal/logger"
)

// Proxy управляет проксированием запросов к бэкендам.
type Proxy struct{}

// NewProxy создает новый экземпляр прокси.
func NewProxy() *Proxy {
	return &Proxy{}
}

// Forward проксирует запрос к указанному URL бэкенда.
func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request, backendURL string) error {
	u, err := url.Parse(backendURL)
	if err != nil {
		logger.ErrorKV("Failed to parse backend URL", "url", backendURL, "error", err)
		return fmt.Errorf("failed to parse backend URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	// Сохраняем исходный ResponseWriter для проверки статуса
	var recorder *httptest.ResponseRecorder
	if rw, ok := w.(*httptest.ResponseRecorder); ok {
		recorder = rw
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.WarnKV("Proxy error", "url", backendURL, "method", r.Method, "error", err)
		w.WriteHeader(http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)

	// Проверяем, был ли записан код ошибки
	if recorder != nil && recorder.Code >= 400 {
		return fmt.Errorf("proxy failed with status %d", recorder.Code)
	}
	return nil
}

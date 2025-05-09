package httpclient

import (
	"net/http"
	"time"
)

// NewClient creates a new HTTP client with the specified timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

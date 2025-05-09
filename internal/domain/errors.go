package domain

import "errors"

var (
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrNoAvailableBackends = errors.New("no available backends")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrInvalidClientConfig = errors.New("invalid client configuration")
	ErrClientNotFound      = errors.New("client not found")
)

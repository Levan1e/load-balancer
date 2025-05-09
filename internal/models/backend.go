package models

import "time"

// Backend represents a backend server.
type Backend struct {
	URL           string
	Healthy       bool
	LastChecked   time.Time
	LoggedHealthy bool // Tracks if healthy status was logged
}

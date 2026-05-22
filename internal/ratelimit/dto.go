// Package ratelimit implements the rate-limited request API (POST /request, GET /stats).
package ratelimit

import (
	"encoding/json"
	"time"
)

// RequestBody is the expected JSON body for POST /request.
type RequestBody struct {
	UserID  string          `json:"user_id"`
	Payload json.RawMessage `json:"payload"`
}

// AcceptedResponse is returned with 201 when a request is accepted.
type AcceptedResponse struct {
	Status     string    `json:"status"`
	UserID     string    `json:"user_id"`
	AcceptedAt time.Time `json:"accepted_at"`
}

// UserStats holds per-user statistics returned by GET /stats.
type UserStats struct {
	AcceptedInCurrentWindow int       `json:"accepted_in_current_window"`
	RejectedTotal           int       `json:"rejected_total"`
	WindowStartedAt         time.Time `json:"window_started_at"`
}

// GlobalStats holds aggregate counters across all users.
type GlobalStats struct {
	TotalAccepted int `json:"total_accepted"`
	TotalRejected int `json:"total_rejected"`
}

// StatsResponse is the response body for GET /stats.
type StatsResponse struct {
	Users  map[string]UserStats `json:"users"`
	Global GlobalStats          `json:"global"`
}

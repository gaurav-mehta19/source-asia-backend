// Package httpx provides helpers for writing consistent JSON HTTP responses.
package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSON encodes v as JSON and writes it to w with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorBody is the standard error envelope returned to clients.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// WriteErrorBody writes a JSON error response with the given error code and message.
func WriteErrorBody(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorBody{Error: code, Message: message})
}

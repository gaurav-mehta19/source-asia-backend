package httpx

import (
	"errors"
	"net/http"
	"strconv"

	domainerrors "github.com/source-asia/backend/internal/shared/errors"
)

// WriteError maps a domain error to an HTTP status code + JSON body.
// It inspects the error chain via errors.Is and errors.As so wrapped errors work correctly.
// Internal error details are never exposed to clients.
func WriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domainerrors.ErrRateLimited):
		var rl *domainerrors.RateLimitedError
		if errors.As(err, &rl) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfterSeconds))
			type rlBody struct {
				Error             string `json:"error"`
				Message           string `json:"message"`
				RetryAfterSeconds int    `json:"retry_after_seconds"`
			}
			WriteJSON(w, http.StatusTooManyRequests, rlBody{
				Error:             "rate_limit_exceeded",
				Message:           rl.Message,
				RetryAfterSeconds: rl.RetryAfterSeconds,
			})
			return
		}
		WriteErrorBody(w, http.StatusTooManyRequests, "rate_limit_exceeded", "rate limit exceeded")

	case errors.Is(err, domainerrors.ErrNotFound):
		WriteErrorBody(w, http.StatusNotFound, "not_found", "resource not found")

	case errors.Is(err, domainerrors.ErrSKUConflict):
		WriteErrorBody(w, http.StatusConflict, "sku_conflict", "sku already exists")

	case errors.Is(err, domainerrors.ErrValidation):
		var ve *domainerrors.ValidationError
		msg := "invalid request"
		if errors.As(err, &ve) {
			msg = ve.Message
		}
		WriteErrorBody(w, http.StatusBadRequest, "validation_error", msg)

	case errors.Is(err, domainerrors.ErrInvalidJSON):
		WriteErrorBody(w, http.StatusBadRequest, "invalid_json", "request body contains invalid JSON")

	default:
		WriteErrorBody(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
	}
}

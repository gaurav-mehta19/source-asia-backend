// Package middleware contains HTTP middleware used by all routes.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/source-asia/backend/internal/shared/httpx"
)

// Recovery catches panics, logs the stack trace, and returns a 500 response.
func Recovery(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
					)
					httpx.WriteErrorBody(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"
	"strings"

	"github.com/source-asia/backend/internal/shared/httpx"
)

// ContentTypeJSON enforces that POST and PUT requests declare Content-Type: application/json.
func ContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				httpx.WriteErrorBody(w, http.StatusUnsupportedMediaType,
					"unsupported_media_type", "Content-Type must be application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

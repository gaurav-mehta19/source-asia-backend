package ratelimit

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/source-asia/backend/internal/config"
)

// RegisterRoutes wires the rate-limit module into the given router.
func RegisterRoutes(r chi.Router, cfg *config.Config, log *slog.Logger) {
	limiter := NewLimiter(cfg.RateLimitMaxRequests, cfg.RateLimitWindowSeconds)
	repo := newMemRepository()
	svc := NewService(limiter, repo)
	ctrl := NewController(svc, log)

	r.Post("/request", ctrl.HandleRequest)
	r.Get("/stats", ctrl.HandleStats)
}

// Package server wires the HTTP server and route registration.
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/source-asia/backend/internal/config"
	appmiddleware "github.com/source-asia/backend/internal/middleware"
	"github.com/source-asia/backend/internal/product"
	"github.com/source-asia/backend/internal/ratelimit"
)

// NewRouter builds the chi router with all middleware and registered routes.
// Middleware order (outermost → innermost):
//  1. Recovery  — catches panics before anything logs
//  2. RequestID — ID must exist before Logger reads it
//  3. Logger    — logs the completed request with ID + status
//  4. ContentTypeJSON — enforced only on POST/PUT
func NewRouter(cfg *config.Config, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(appmiddleware.Recovery(log))
	r.Use(appmiddleware.RequestID)
	r.Use(appmiddleware.Logger(log))
	r.Use(appmiddleware.ContentTypeJSON)
	r.Use(chimiddleware.StripSlashes)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	ratelimit.RegisterRoutes(r, cfg, log)
	product.RegisterRoutes(r, cfg, log)

	return r
}

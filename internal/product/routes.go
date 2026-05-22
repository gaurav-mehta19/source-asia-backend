package product

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/source-asia/backend/internal/config"
)

// RegisterRoutes wires the product module into the given router under /products.
func RegisterRoutes(r chi.Router, cfg *config.Config, log *slog.Logger) {
	repo := NewRepository(cfg.ProductMaxMediaPerProduct)
	svc := NewService(repo,
		cfg.ProductMaxURLsPerRequest,
		cfg.ProductMaxURLLength,
		cfg.ProductDefaultPageLimit,
		cfg.ProductMaxPageLimit,
	)
	ctrl := NewController(svc, log)

	r.Route("/products", func(r chi.Router) {
		r.Post("/", ctrl.HandleCreate)
		r.Get("/", ctrl.HandleList)
		r.Get("/{id}", ctrl.HandleGetByID)
		r.Post("/{id}/media", ctrl.HandleAddMedia)
	})
}

package product

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	domainerrors "github.com/source-asia/backend/internal/shared/errors"
	"github.com/source-asia/backend/internal/shared/httpx"
)

// Controller handles HTTP requests for the product catalog API.
type Controller struct {
	svc Service
	log *slog.Logger
}

// NewController creates a Controller with the given service and logger.
func NewController(svc Service, log *slog.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// HandleCreate handles POST /products.
func (c *Controller) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, domainerrors.ErrInvalidJSON)
		return
	}

	resp, err := c.svc.Create(r.Context(), &req)
	if err != nil {
		c.log.Info("create product failed", "error", err.Error())
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// HandleList handles GET /products.
func (c *Controller) HandleList(w http.ResponseWriter, r *http.Request) {
	limit, err := parseQueryInt(r, "limit", 0)
	if err != nil {
		httpx.WriteError(w, domainerrors.NewValidation("limit must be a non-negative integer"))
		return
	}
	offset, err := parseQueryInt(r, "offset", 0)
	if err != nil {
		httpx.WriteError(w, domainerrors.NewValidation("offset must be a non-negative integer"))
		return
	}

	resp, err := c.svc.List(r.Context(), limit, offset)
	if err != nil {
		c.log.Error("list products failed", "error", err.Error())
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

// HandleGetByID handles GET /products/{id}.
func (c *Controller) HandleGetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resp, err := c.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

// HandleAddMedia handles POST /products/{id}/media.
func (c *Controller) HandleAddMedia(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req AddMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, domainerrors.ErrInvalidJSON)
		return
	}

	resp, err := c.svc.AddMedia(r.Context(), id, &req)
	if err != nil {
		c.log.Info("add media failed", "product_id", id, "error", err.Error())
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func parseQueryInt(r *http.Request, key string, fallback int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

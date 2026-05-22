package ratelimit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	domainerrors "github.com/source-asia/backend/internal/shared/errors"
	"github.com/source-asia/backend/internal/shared/httpx"
)

// Controller handles HTTP requests for the rate-limit API.
type Controller struct {
	svc Service
	log *slog.Logger
}

// NewController creates a Controller with the given service and logger.
func NewController(svc Service, log *slog.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// HandleRequest handles POST /request.
func (c *Controller) HandleRequest(w http.ResponseWriter, r *http.Request) {
	var body RequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, domainerrors.ErrInvalidJSON)
		return
	}

	// Input validation.
	body.UserID = strings.TrimSpace(body.UserID)
	if body.UserID == "" {
		httpx.WriteError(w, domainerrors.NewValidation("user_id is required and must not be empty"))
		return
	}
	if len(body.UserID) > 256 {
		httpx.WriteError(w, domainerrors.NewValidation("user_id must not exceed 256 characters"))
		return
	}
	if len(body.Payload) == 0 {
		httpx.WriteError(w, domainerrors.NewValidation("payload is required"))
		return
	}
	// Verify payload is valid JSON (any value including null/object/array).
	if !json.Valid(body.Payload) {
		httpx.WriteError(w, domainerrors.NewValidation("payload must be valid JSON"))
		return
	}

	resp, err := c.svc.ProcessRequest(r.Context(), body.UserID)
	if err != nil {
		c.log.Info("request rejected", "user_id", body.UserID, "error", err.Error())
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// HandleStats handles GET /stats.
func (c *Controller) HandleStats(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))

	resp, err := c.svc.GetStats(r.Context(), userID)
	if err != nil {
		c.log.Error("stats error", "error", err.Error())
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

package ratelimit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func newTestController() *Controller {
	limiter := NewLimiter(5, 60)
	repo := newMemRepository()
	svc := NewService(limiter, repo)
	return NewController(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func postRequest(t *testing.T, ctrl *Controller, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ctrl.HandleRequest(rr, req)
	return rr
}

// TestHandleRequest_201OnAccept covers the happy path including response body shape.
func TestHandleRequest_201OnAccept(t *testing.T) {
	ctrl := newTestController()
	rr := postRequest(t, ctrl, `{"user_id":"alice","payload":{"hello":"world"}}`)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp AcceptedResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "accepted" || resp.UserID != "alice" {
		t.Fatalf("unexpected response body: %+v", resp)
	}
}

// TestHandleRequest_429AfterBoundary verifies the 6th request is rejected with the right status.
func TestHandleRequest_429AfterBoundary(t *testing.T) {
	ctrl := newTestController()
	for i := 0; i < 5; i++ {
		rr := postRequest(t, ctrl, `{"user_id":"alice","payload":{"n":1}}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("setup request %d: got %d", i, rr.Code)
		}
	}
	rr := postRequest(t, ctrl, `{"user_id":"alice","payload":{"n":1}}`)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

// TestHandleRequest_RetryAfterHeaderIsInteger guards against regression of Bug 1:
// Retry-After must be an integer string (RFC 7231), not "Too Many Requests".
func TestHandleRequest_RetryAfterHeaderIsInteger(t *testing.T) {
	ctrl := newTestController()
	for i := 0; i < 5; i++ {
		postRequest(t, ctrl, `{"user_id":"alice","payload":1}`)
	}
	rr := postRequest(t, ctrl, `{"user_id":"alice","payload":1}`)

	hdr := rr.Header().Get("Retry-After")
	if hdr == "" {
		t.Fatal("Retry-After header missing on 429")
	}
	n, err := strconv.Atoi(hdr)
	if err != nil {
		t.Fatalf("Retry-After must be integer seconds, got %q (parse error: %v)", hdr, err)
	}
	if n < 1 || n > 60 {
		t.Fatalf("Retry-After out of expected range [1,60]: %d", n)
	}
}

// TestHandleRequest_400OnInvalidInput covers the spec's required 400 cases.
func TestHandleRequest_400OnInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing user_id", `{"payload":42}`},
		{"empty user_id", `{"user_id":"","payload":42}`},
		{"whitespace user_id", `{"user_id":"   ","payload":42}`},
		{"missing payload", `{"user_id":"alice"}`},
		{"malformed JSON", `{"user_id":"alice","payload":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := newTestController()
			rr := postRequest(t, ctrl, tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// TestHandleStats_ReturnsPerUserAndGlobal verifies the documented JSON schema.
func TestHandleStats_ReturnsPerUserAndGlobal(t *testing.T) {
	ctrl := newTestController()
	for i := 0; i < 5; i++ {
		postRequest(t, ctrl, `{"user_id":"alice","payload":1}`)
	}
	// 2 rejected
	postRequest(t, ctrl, `{"user_id":"alice","payload":1}`)
	postRequest(t, ctrl, `{"user_id":"alice","payload":1}`)

	req := httptest.NewRequest(http.MethodGet, "/stats?user_id=alice", nil)
	rr := httptest.NewRecorder()
	ctrl.HandleStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp StatsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stats := resp.Users["alice"]
	if stats.AcceptedInCurrentWindow != 5 {
		t.Fatalf("expected 5 accepted in window, got %d", stats.AcceptedInCurrentWindow)
	}
	if stats.RejectedTotal != 2 {
		t.Fatalf("expected 2 rejected total, got %d", stats.RejectedTotal)
	}
	if resp.Global.TotalAccepted != 5 || resp.Global.TotalRejected != 2 {
		t.Fatalf("unexpected global totals: %+v", resp.Global)
	}
}

// Compile-time check: Service interface is satisfied by *service.
var _ Service = (Service)(nil)

// silence "unused" complaints if context import drifts.
var _ = context.Background

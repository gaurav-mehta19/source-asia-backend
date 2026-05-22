package product

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestServer() http.Handler {
	return newTestServerWithCap(200)
}

func newTestServerWithCap(maxMediaPerProduct int) http.Handler {
	repo := NewRepository(maxMediaPerProduct)
	svc := NewService(repo, 20, 2048, 20, 100)
	ctrl := NewController(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := chi.NewRouter()
	r.Route("/products", func(r chi.Router) {
		r.Post("/", ctrl.HandleCreate)
		r.Get("/", ctrl.HandleList)
		r.Get("/{id}", ctrl.HandleGetByID)
		r.Post("/{id}/media", ctrl.HandleAddMedia)
	})
	return r
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

func TestPOSTProducts_201AndDuplicateSKU409(t *testing.T) {
	h := newTestServer()

	rr := do(t, h, http.MethodPost, "/products",
		`{"name":"Widget","sku":"SKU-1","image_urls":["https://cdn.example.com/a.jpg"]}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var first DetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &first)
	if first.ID == "" {
		t.Fatal("expected non-empty id in response")
	}

	rr = do(t, h, http.MethodPost, "/products", `{"name":"Other","sku":"SKU-1"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate sku, got %d", rr.Code)
	}
}

func TestPOSTProducts_400OnValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":"","sku":"S"}`},
		{"empty sku", `{"name":"N","sku":""}`},
		{"bad URL scheme", `{"name":"N","sku":"S","image_urls":["ftp://x.example/a"]}`},
		{"too many URLs", `{"name":"N","sku":"S","image_urls":` + manyURLs(21) + `}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, newTestServer(), http.MethodPost, "/products", tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body=%s)", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestGETProductsList_HidesURLArrays(t *testing.T) {
	h := newTestServer()
	do(t, h, http.MethodPost, "/products",
		`{"name":"W","sku":"S1","image_urls":["https://cdn.example.com/a.jpg","https://cdn.example.com/b.jpg"]}`)

	rr := do(t, h, http.MethodGet, "/products?limit=10&offset=0", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if strings.Contains(body, "image_urls") || strings.Contains(body, "video_urls") {
		t.Fatalf("list response must not include full URL arrays, body=%s", body)
	}

	var resp ListResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].ImageCount != 2 {
		t.Fatalf("unexpected list payload: %+v", resp)
	}
	if resp.Pagination.Total != 1 || resp.Pagination.HasMore {
		t.Fatalf("unexpected pagination: %+v", resp.Pagination)
	}
}

func TestGETProductByID_FullPayloadAnd404(t *testing.T) {
	h := newTestServer()
	rr := do(t, h, http.MethodPost, "/products",
		`{"name":"W","sku":"S1","image_urls":["https://cdn.example.com/a.jpg"]}`)
	var created DetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	rr = do(t, h, http.MethodGet, "/products/"+created.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var full DetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &full)
	if len(full.ImageURLs) != 1 {
		t.Fatalf("expected 1 image URL in detail, got %d", len(full.ImageURLs))
	}

	rr = do(t, h, http.MethodGet, "/products/does-not-exist", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestPOSTAddMedia_400OnEmptyBody_404OnUnknownID(t *testing.T) {
	h := newTestServer()
	rr := do(t, h, http.MethodPost, "/products", `{"name":"W","sku":"S1"}`)
	var created DetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	// Empty body — both arrays missing/empty.
	rr = do(t, h, http.MethodPost, "/products/"+created.ID+"/media", `{}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty body, got %d (body=%s)", rr.Code, rr.Body.String())
	}

	// Unknown id.
	rr = do(t, h, http.MethodPost, "/products/nope/media",
		`{"image_urls":["https://cdn.example.com/a.jpg"]}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// TestPOSTAddMedia_400WhenExceedingPerProductCap covers the new cap enforcement at the HTTP layer.
func TestPOSTAddMedia_400WhenExceedingPerProductCap(t *testing.T) {
	h := newTestServerWithCap(3)

	rr := do(t, h, http.MethodPost, "/products",
		`{"name":"W","sku":"S1","image_urls":["https://cdn.example.com/a.jpg","https://cdn.example.com/b.jpg"]}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup create: expected 201, got %d", rr.Code)
	}
	var created DetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	// Cap is 3, already 2 images. Adding 2 more (total = 4) → 400.
	rr = do(t, h, http.MethodPost, "/products/"+created.ID+"/media",
		`{"image_urls":["https://cdn.example.com/c.jpg","https://cdn.example.com/d.jpg"]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 over cap, got %d (body=%s)", rr.Code, rr.Body.String())
	}

	// Adding exactly 1 more (total = 3) → 200.
	rr = do(t, h, http.MethodPost, "/products/"+created.ID+"/media",
		`{"image_urls":["https://cdn.example.com/c.jpg"]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 at exact cap, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func manyURLs(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = `"https://cdn.example.com/x.jpg"`
	}
	return "[" + strings.Join(parts, ",") + "]"
}

# Source Asia Backend

A production-grade Go HTTP API implementing a rate-limited request endpoint and a product catalog, built for the Source Asia Backend Assignment.

**Live demo:** https://source-asia-backend-lbl5.onrender.com

```bash
curl https://source-asia-backend-lbl5.onrender.com/health
# → {"status":"ok"}
```

> **Hosted on Render's free tier.** The service sleeps after 15 min of inactivity. During the ~10–30 s spin-up window, Render's edge proxy returns a plaintext `404 Not Found` (look for the `x-render-routing: no-server` header — that 404 is from Render, not this service). **Retry once after a few seconds** and you'll get a real response with `x-render-origin-server: Render` in the headers. Subsequent calls are sub-second. In-memory state (rate-limit counters, products) also resets on cold start — see [Production Limitations](#9-production-limitations--migration-path).

---

## 1. Overview

**Part 1 — Rate-Limited API**

- `POST /request` — accepts or rejects user requests using a sliding-window rate limiter (5 requests per user per 60-second window).
- `GET /stats` — returns per-user and global accept/reject counters.

**Part 2 — Product Catalog**

- `POST /products` — create a product with optional media URLs.
- `GET /products` — paginated list with image/video counts and thumbnail (no full URL arrays).
- `GET /products/{id}` — full product detail including all media URLs.
- `POST /products/{id}/media` — append image/video URLs to an existing product.

---

## 2. Architecture

```
HTTP Request
     │
     ▼
┌─────────────────────────────┐
│  Middleware Chain           │  Recovery → RequestID → Logger → ContentTypeJSON
└────────────┬────────────────┘
             │
             ▼
┌─────────────────────────────┐
│  Controller                 │  Decode JSON, validate input shape, map errors to HTTP
└────────────┬────────────────┘
             │ calls via interface
             ▼
┌─────────────────────────────┐
│  Service                    │  Business rules, orchestration, domain error production
└────────────┬────────────────┘
             │ calls via interface
             ▼
┌─────────────────────────────┐
│  Repository                 │  In-memory storage with sync primitives
└─────────────────────────────┘
```

**Why this structure?**

- Each layer depends only on the interface of the layer below — easy to swap implementations (e.g., replace in-memory repo with Postgres).
- Domain errors bubble up from repository/service; controllers map them to HTTP status codes in one place (`httpx.WriteError`).
- No globals except config, which is loaded once at startup and passed via constructors.

---

## 3. Project Structure

```
source-asia-backend/
├── cmd/server/main.go              # Entrypoint: load config, wire deps, start server
├── internal/
│   ├── config/config.go            # .env loading, validation, typed Config struct
│   ├── logger/logger.go            # slog setup (JSON in prod, text in dev)
│   ├── middleware/
│   │   ├── recovery.go             # Panic recovery → 500
│   │   ├── request_id.go           # UUID request ID header + context
│   │   ├── logger.go               # Request logging (method, path, status, duration)
│   │   └── content_type.go         # Enforce application/json on POST/PUT
│   ├── ratelimit/
│   │   ├── limiter.go              # Rolling-window limiter, per-user mutex sharding
│   │   ├── repository.go           # Atomic global counters
│   │   ├── service.go              # Business logic (ProcessRequest, GetStats)
│   │   ├── controller.go           # HTTP handlers
│   │   ├── dto.go                  # Request/response DTOs
│   │   └── routes.go               # Route registration
│   ├── product/
│   │   ├── model.go                # Product + Media domain types
│   │   ├── repository.go           # Separate maps for core/media/counts/thumbnails
│   │   ├── validator.go            # URL and field validation
│   │   ├── service.go              # Business logic (Create, List, GetByID, AddMedia)
│   │   ├── controller.go           # HTTP handlers
│   │   ├── dto.go                  # ListItem vs DetailResponse DTOs
│   │   └── routes.go               # Route registration
│   └── shared/
│       ├── errors/errors.go        # Typed domain errors
│       ├── httpx/
│       │   ├── response.go         # WriteJSON, WriteErrorBody
│       │   └── errors.go           # Domain error → HTTP status mapping
│       └── validator/url.go        # Reusable URL validation
├── .env.example                    # Sample env (committed)
├── .env                            # Actual env (gitignored)
├── Makefile
└── README.md
```

---

## 4. How to Run

```bash
# Clone / extract the project
cd source-asia-backend

# Copy the sample env
cp .env.example .env

# Install dependencies (already in go.sum)
go mod download

# Run the server
go run ./cmd/server
# or
make run

# Build a binary
make build
./bin/server

# Run the test suite (race detector enabled)
make test
```

The server starts on port `8080` by default (configurable via `HTTP_PORT`).

---

## 5. API Documentation

### Health Check

```bash
curl http://localhost:8080/health
# 200 {"status":"ok"}
```

---

### POST /request

Accept or reject a user request based on the rate limit.

**Request:**
```bash
curl -s -X POST http://localhost:8080/request \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "payload": {"action": "buy", "item": 42}}'
```

**201 Accepted:**
```json
{
  "status": "accepted",
  "user_id": "alice",
  "accepted_at": "2026-05-20T10:30:00Z"
}
```

**429 Rate Limited:**
```json
{
  "error": "rate_limit_exceeded",
  "message": "user has exceeded 5 requests per minute",
  "retry_after_seconds": 47
}
```

**400 Validation Error:**
```json
{
  "error": "validation_error",
  "message": "user_id is required and must not be empty"
}
```

---

### GET /stats

**All users:**
```bash
curl http://localhost:8080/stats
```

**Single user:**
```bash
curl "http://localhost:8080/stats?user_id=alice"
```

**Response:**
```json
{
  "users": {
    "alice": {
      "accepted_in_current_window": 3,
      "rejected_total": 2,
      "window_started_at": "2026-05-20T10:30:00Z"
    }
  },
  "global": {
    "total_accepted": 3,
    "total_rejected": 2
  }
}
```

---

### POST /products

```bash
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Widget A",
    "sku": "SKU-001",
    "image_urls": ["https://cdn.example.com/img1.jpg"],
    "video_urls": ["https://cdn.example.com/vid1.mp4"]
  }'
```

**201 Created:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Widget A",
  "sku": "SKU-001",
  "image_urls": ["https://cdn.example.com/img1.jpg"],
  "video_urls": ["https://cdn.example.com/vid1.mp4"],
  "created_at": "2026-05-20T10:00:00Z",
  "updated_at": "2026-05-20T10:00:00Z"
}
```

**409 SKU Conflict:**
```json
{"error": "sku_conflict", "message": "sku already exists"}
```

---

### GET /products

```bash
curl "http://localhost:8080/products?limit=10&offset=0"
```

**200 OK:**
```json
{
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Widget A",
      "sku": "SKU-001",
      "image_count": 1,
      "video_count": 1,
      "thumbnail_url": "https://cdn.example.com/img1.jpg",
      "created_at": "2026-05-20T10:00:00Z"
    }
  ],
  "pagination": {
    "limit": 10,
    "offset": 0,
    "total": 1,
    "has_more": false
  }
}
```

---

### GET /products/{id}

```bash
curl http://localhost:8080/products/550e8400-e29b-41d4-a716-446655440000
```

**200 OK:** (full detail with URL arrays)

**404 Not Found:**
```json
{"error": "not_found", "message": "resource not found"}
```

---

### POST /products/{id}/media

```bash
curl -s -X POST http://localhost:8080/products/550e8400-e29b-41d4-a716-446655440000/media \
  -H "Content-Type: application/json" \
  -d '{
    "image_urls": ["https://cdn.example.com/img2.jpg", "https://cdn.example.com/img3.jpg"]
  }'
```

**200 OK:** (returns updated DetailResponse)

---

## 6. Rate Limiter Design

### Rolling Window (Sliding Window Log)

For each user, we maintain a log of accepted-request timestamps (not counters). On each incoming request:

1. Drop timestamps older than `window_duration` (60 s by default) from the front of the log.
2. If `len(log) < maxRequests` (5), append the current timestamp and **accept**.
3. Otherwise, increment the cumulative rejected counter and **reject**.

This is a true sliding window — it never has the "boundary burst" problem of fixed windows (where a user can fire 5 requests at 0:59 and 5 more at 1:01).

### Concurrency Strategy — Per-User Mutex Sharding

```
globalMu (sync.Mutex)
    └── protects: users map[string]*userState

userState.mu (sync.Mutex, one per user)
    └── protects: windowTimes []time.Time, rejectedTotal int
```

**Two-phase locking:**
1. Acquire `globalMu` to look up (or create) the `*userState` pointer — release immediately.
2. Acquire `userState.mu` to perform the atomic check-and-record.

This means:
- 1000 concurrent requests for **different users** → zero contention after the map lookup.
- 1000 concurrent requests for the **same user** → serialised only at the per-user mutex, not globally.
- The check-and-record is a **single critical section** — no read-then-write race possible.

The rejected counter is stored inside `userState` (incremented under the per-user lock) and **also** mirrored to the global `Repository` via atomic integers for the `global` section of `/stats`.

---

## 7. Product Storage Design

### Five Separate Maps

| Map | Key | Value | Used by |
|-----|-----|-------|---------|
| `products` | product ID | `*Product` (core fields) | List + Detail |
| `media` | product ID | `*Media` (full URL slices) | Detail **only** |
| `skuIndex` | SKU | product ID | Create (uniqueness) |
| `mediaCounts` | product ID | `{ImageCount, VideoCount int}` | List **only** |
| `thumbnails` | product ID | first image URL string | List **only** |
| `sortedIDs` | — | `[]string` (insertion order) | List (pagination) |

### Why List Never Touches `media`

The `List` repository method is:

```go
for _, id := range r.sortedIDs[offset:end] {
    p := r.products[id]          // small struct — 5 fields
    counts := r.mediaCounts[id]  // two ints
    thumb := r.thumbnails[id]    // one string pointer
    // ... build ListItem
}
```

It **never** reads `r.media[id]`. The `media` map holds potentially large `[]string` slices (up to 20 URLs × 2048 chars each). For 1000 products × 20 image URLs, that's ~40 MB that the list endpoint completely avoids allocating and serialising.

### Complexity

| Operation | Time | Notes |
|-----------|------|-------|
| Create | O(1) | Map inserts + slice append |
| List | O(limit) | Slice subrange + map lookups |
| GetByID | O(n) URL copy | Where n = total URLs for product |
| AddMedia | O(1) | Append + update counts |

---

## 8. Validation Rules

| Field | Rule |
|-------|------|
| `user_id` | Required, non-empty, max 256 chars |
| `payload` | Required, must be valid JSON (any value) |
| Product `name` | Required, non-empty, max 200 chars |
| Product `sku` | Required, non-empty, max 100 chars, globally unique |
| `image_urls` / `video_urls` | Max 20 per request; each URL: http/https scheme, host present, ≤ 2048 chars |
| Pagination `limit` | Default 20, max 100, must be non-negative integer |
| Pagination `offset` | Default 0, must be non-negative integer |
| `POST /products/{id}/media` | At least one of `image_urls` / `video_urls` must be non-empty |
| Per-product media cap | Combined `image_urls + video_urls` ≤ `PRODUCT_MAX_MEDIA_PER_PRODUCT` (default **200**). Enforced atomically inside the repository write lock, so concurrent appends cannot collectively breach the cap. Exceeding the cap returns **400 Bad Request**. |

---

## 9. Production Limitations & Migration Path

### Current Limitations

- **Single instance only**: rate-limit state and product data live in process memory. A restart loses all data. Horizontal scaling would give each instance independent counters — violating the "5 per user" guarantee.
- **No persistence**: no database backing.
- **No CDN**: media URLs are stored as-is; no transformation pipeline.
- **Render free-tier cold start**: the live deployment sleeps after 15 min of inactivity. During the spin-up window Render's edge serves a plaintext `404 Not Found` (header `x-render-routing: no-server`) — this is **not a missing route**, it's the upstream not being ready yet. A retry after a few seconds reaches the warm container (header `x-render-origin-server: Render`). On a paid tier or with a 5-min uptime ping this disappears.
- **Per-product media cap is a hard limit, not a paginated structure**: combined `image_urls + video_urls` is bounded by `PRODUCT_MAX_MEDIA_PER_PRODUCT` (default 200). A real catalog with thousands of variant images per product would store media in a separate paginated table (see Postgres schema below) and serve it via a dedicated `GET /products/{id}/media?cursor=…` endpoint rather than embedding the full array in the detail response.
- **No DELETE / PATCH endpoints**: products and media URLs cannot be removed or reordered. The `sortedIDs` slice that backs list pagination is append-only.

### Migration to Postgres + Redis + CDN

**Rate Limiting → Redis**

Replace `Limiter` with Redis sorted sets (sliding window log):

```
ZADD user:{id}:requests {now_ms} {now_ms}
ZREMRANGEBYSCORE user:{id}:requests -inf {cutoff_ms}
count = ZCARD user:{id}:requests
if count < 5: EXPIRE user:{id}:requests 60  → accept
else: reject
```

Or use `INCR` + `EXPIRE` for fixed-window (simpler, slightly less accurate).

**Product Catalog → Postgres**

```sql
CREATE TABLE products (
  id         UUID PRIMARY KEY,
  name       VARCHAR(200) NOT NULL,
  sku        VARCHAR(100) NOT NULL UNIQUE,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE product_media (
  id         BIGSERIAL PRIMARY KEY,
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  media_type VARCHAR(10) NOT NULL CHECK (media_type IN ('image','video')),
  url        VARCHAR(2048) NOT NULL,
  position   INT NOT NULL DEFAULT 0
);

CREATE INDEX ON product_media(product_id, media_type);
```

List query (never fetches URLs):
```sql
SELECT p.id, p.name, p.sku, p.created_at,
       COUNT(*) FILTER (WHERE m.media_type='image') AS image_count,
       COUNT(*) FILTER (WHERE m.media_type='video') AS video_count,
       MIN(m.url) FILTER (WHERE m.media_type='image' AND m.position=0) AS thumbnail_url
FROM products p
LEFT JOIN product_media m ON m.product_id = p.id
GROUP BY p.id
ORDER BY p.created_at
LIMIT $1 OFFSET $2;
```

**Media → CDN**

Store a CDN origin path in Postgres; serve via Cloudflare/CloudFront. The application layer generates signed URLs on read if needed.

**Code changes required:**
- Replace `memRepository` with a Postgres-backed implementation of the same `Repository` interface — controllers and services remain unchanged.
- Replace `Limiter` with a Redis-backed implementation of an `Allower` interface.

---

## 10. Concurrency Notes

| Resource | Protected by | Reason |
|----------|-------------|--------|
| `users map` (rate limiter) | `Limiter.globalMu` (Mutex) | Map writes need exclusive lock; held only for pointer lookup |
| `userState.windowTimes`, `rejectedTotal` | `userState.mu` (Mutex) | Per-user critical section; no global bottleneck |
| `totalAccepted`, `totalRejected` (repo) | `atomic.Int64` | Single-word updates are cheaper than a mutex for high-frequency increments |
| `products`, `media`, `skuIndex`, `mediaCounts`, `thumbnails`, `sortedIDs` | `memRepository.mu` (RWMutex) | Multiple readers OK; writer gets exclusive lock |

`AddMedia` performs the existence check, per-product cap check, append, count update, thumbnail-cache update, **and** read-back of the resulting `DetailResponse` under a single write lock — there is no TOCTOU window between mutation and the response a caller observes. Concurrent appends therefore cannot collectively breach the per-product cap, and the returned slices are defensive copies so a caller mutating them cannot corrupt storage.

**No data race**: verified by Go's race detector (`go test -race`).

**Why no global lock for the rate limiter?** A single mutex protecting all users would serialise every request on the server — catastrophic at high concurrency. Per-user sharding limits lock scope to exactly those requests competing for the same user's quota.

---

## 11. AI Tools Disclosure

This project was implemented with the assistance of **Claude Code** (Anthropic), which was used to:

- Generate the initial implementation of all Go source files following the provided specification.
- Wire dependencies and validate the architecture against the layered design rules.
- Produce the README documentation.

All generated code was reviewed for correctness, concurrency safety, and alignment with the specification.

---

## 12. Testing

### Automated Tests

The repo ships with **24 unit and HTTP-level tests** covering the rate limiter (boundary, retry-after, sliding-window eviction, concurrent same-user, concurrent different-users), the product repository (list/detail split, defensive slice copy, pagination, thumbnail caching, `UpdatedAt` updates), and the HTTP handlers (status codes, validation, headers).

```bash
# Run everything with the race detector
make test

# Equivalent
go test -race -count=1 ./...

# Single package, verbose
go test -race -v ./internal/ratelimit
go test -race -v ./internal/product

# Single test (e.g. the 1000-goroutine concurrency proof)
go test -race -run TestAllow_ConcurrentSameUser ./internal/ratelimit
```

Notable tests:

| Test | What it proves |
|---|---|
| `TestAllow_ConcurrentSameUser` | 1000 goroutines for one user → exactly 5 accepts (concurrency requirement) |
| `TestAllow_ConcurrentDifferentUsers` | 200 users × 5 requests in parallel → all 1000 accepted (per-user mutex sharding works) |
| `TestHandleRequest_RetryAfterHeaderIsInteger` | `Retry-After` is RFC 7231-compliant integer seconds |
| `TestList_DoesNotReturnURLArrays` | List response body contains no `image_urls` / `video_urls` keys (performance rule) |
| `TestAddMedia_UpdatesUpdatedAt` | `UpdatedAt` advances every time media is appended |
| `TestGetByID_CopiesSlices` | Mutating a returned slice cannot leak into storage |

### Manual / curl Tests

#### Rate Limit — Trigger 429

```bash
# Send 7 requests for the same user; requests 6 and 7 should be 429
for i in $(seq 1 7); do
  curl -s -X POST http://localhost:8080/request \
    -H "Content-Type: application/json" \
    -d '{"user_id":"testuser","payload":{"n":'$i'}}' | jq -r '.status // .error'
done
```

Expected output: `accepted accepted accepted accepted accepted rate_limit_exceeded rate_limit_exceeded`

#### SKU Conflict — 409

```bash
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"A","sku":"DUPE"}' | jq .

curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"B","sku":"DUPE"}' | jq .
# → {"error":"sku_conflict","message":"sku already exists"}
```

#### Pagination

```bash
# Create 3 products then paginate
for sku in P1 P2 P3; do
  curl -s -X POST http://localhost:8080/products \
    -H "Content-Type: application/json" \
    -d '{"name":"'$sku'","sku":"'$sku'"}' > /dev/null
done

curl -s "http://localhost:8080/products?limit=2&offset=0" | jq '.pagination'
# → {"limit":2,"offset":0,"total":3,"has_more":true}

curl -s "http://localhost:8080/products?limit=2&offset=2" | jq '.pagination'
# → {"limit":2,"offset":2,"total":3,"has_more":false}
```

#### 404

```bash
curl -s http://localhost:8080/products/nonexistent-id | jq .
# → {"error":"not_found","message":"resource not found"}
```

#### Validation Errors

```bash
# Missing user_id
curl -s -X POST http://localhost:8080/request \
  -H "Content-Type: application/json" \
  -d '{"payload":42}' | jq .

# Invalid URL scheme
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"X","sku":"Y","image_urls":["ftp://bad.url"]}' | jq .

# Both media arrays empty
ID=$(curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"Z","sku":"Z-SKU"}' | jq -r .id)
curl -s -X POST http://localhost:8080/products/$ID/media \
  -H "Content-Type: application/json" \
  -d '{}' | jq .
```

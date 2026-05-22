package product

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainerrors "github.com/source-asia/backend/internal/shared/errors"
)

// mediaCount is a denormalised struct stored separately from the full Media slice.
// The list endpoint reads ONLY this struct — it never touches media[id].ImageURLs
// or media[id].VideoURLs, which may be large slices.
type mediaCount struct {
	ImageCount int
	VideoCount int
}

// Repository defines the storage interface consumed by the service layer.
type Repository interface {
	Create(ctx context.Context, p *Product, m *Media) error
	List(ctx context.Context, limit, offset int) ([]ListItem, int, error)
	GetByID(ctx context.Context, id string) (*DetailResponse, error)
	// AddMedia appends URLs and returns the resulting detail under a single write lock —
	// no TOCTOU window between append and read-back.
	AddMedia(ctx context.Context, id string, imageURLs, videoURLs []string) (*DetailResponse, error)
}

// memRepository is a concurrency-safe in-memory store for the product catalog.
//
// Storage layout (five separate maps for performance):
//
//	products     map[string]*Product      — core fields only; read for list + detail
//	media        map[string]*Media        — full URL slices; read ONLY for GET /products/{id}
//	skuIndex     map[string]string        — SKU → product ID for O(1) uniqueness check
//	mediaCounts  map[string]mediaCount    — denormalised counts; read for GET /products (list)
//	thumbnails   map[string]string        — cached first image URL; read for GET /products (list)
//	sortedIDs    []string                 — insertion-ordered slice for stable pagination
//
// Why separate maps?
//
//	The list endpoint serves potentially hundreds of products. Deserialising full []string
//	slices from media would waste CPU and memory. By keeping counts and the thumbnail in
//	their own tiny structs, the list path is O(limit) reads of small values only.
//
// All fields are protected by a single sync.RWMutex:
//   - Read lock for List, GetByID, AddMedia (read phase).
//   - Write lock for Create and AddMedia (write phase).
type memRepository struct {
	mu                 sync.RWMutex
	products           map[string]*Product
	media              map[string]*Media
	skuIndex           map[string]string
	mediaCounts        map[string]mediaCount
	thumbnails         map[string]string
	sortedIDs          []string
	maxMediaPerProduct int // cap on combined image+video URLs per product
}

// NewRepository returns an initialised in-memory Repository.
// maxMediaPerProduct is the upper bound on combined image+video URL count per product,
// enforced atomically under the write lock by Create and AddMedia.
func NewRepository(maxMediaPerProduct int) Repository {
	return &memRepository{
		products:           make(map[string]*Product),
		media:              make(map[string]*Media),
		skuIndex:           make(map[string]string),
		mediaCounts:        make(map[string]mediaCount),
		thumbnails:         make(map[string]string),
		maxMediaPerProduct: maxMediaPerProduct,
	}
}

func (r *memRepository) Create(_ context.Context, p *Product, m *Media) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if total := len(m.ImageURLs) + len(m.VideoURLs); total > r.maxMediaPerProduct {
		return domainerrors.NewValidation(fmt.Sprintf(
			"total media (image_urls + video_urls = %d) exceeds per-product cap of %d",
			total, r.maxMediaPerProduct))
	}

	if _, exists := r.skuIndex[p.SKU]; exists {
		return domainerrors.ErrSKUConflict
	}

	r.products[p.ID] = p
	r.media[p.ID] = m
	r.skuIndex[p.SKU] = p.ID
	r.mediaCounts[p.ID] = mediaCount{
		ImageCount: len(m.ImageURLs),
		VideoCount: len(m.VideoURLs),
	}
	if len(m.ImageURLs) > 0 {
		r.thumbnails[p.ID] = m.ImageURLs[0]
	}
	r.sortedIDs = append(r.sortedIDs, p.ID)
	return nil
}

// List returns a page of ListItems.
// IMPORTANT: this method reads ONLY products, mediaCounts, and thumbnails maps.
// It never dereferences media[id].ImageURLs or media[id].VideoURLs.
// Complexity: O(limit) — only the requested page is iterated.
func (r *memRepository) List(_ context.Context, limit, offset int) ([]ListItem, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.sortedIDs)
	if offset >= total {
		return []ListItem{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	page := r.sortedIDs[offset:end]
	items := make([]ListItem, 0, len(page))
	for _, id := range page {
		p := r.products[id]
		counts := r.mediaCounts[id]
		items = append(items, ListItem{
			ID:           p.ID,
			Name:         p.Name,
			SKU:          p.SKU,
			ImageCount:   counts.ImageCount,
			VideoCount:   counts.VideoCount,
			ThumbnailURL: r.thumbnails[id],
			CreatedAt:    p.CreatedAt,
		})
	}
	return items, total, nil
}

func (r *memRepository) GetByID(_ context.Context, id string) (*DetailResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.products[id]
	if !ok {
		return nil, domainerrors.ErrNotFound
	}
	m := r.media[id]

	// Copy slices so callers cannot mutate internal state.
	imgs := make([]string, len(m.ImageURLs))
	copy(imgs, m.ImageURLs)
	vids := make([]string, len(m.VideoURLs))
	copy(vids, m.VideoURLs)

	return &DetailResponse{
		ID:        p.ID,
		Name:      p.Name,
		SKU:       p.SKU,
		ImageURLs: imgs,
		VideoURLs: vids,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}, nil
}

// AddMedia appends URLs and returns the resulting DetailResponse atomically.
// Doing both under a single write lock closes the AddMedia→GetByID TOCTOU window:
// callers observe the exact state produced by this call, never a state mutated by
// an interleaving append from another goroutine.
func (r *memRepository) AddMedia(_ context.Context, id string, imageURLs, videoURLs []string) (*DetailResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.products[id]
	if !ok {
		return nil, domainerrors.ErrNotFound
	}
	m := r.media[id]

	// Enforce per-product cap inside the lock so concurrent appends cannot collectively exceed it.
	newTotal := len(m.ImageURLs) + len(m.VideoURLs) + len(imageURLs) + len(videoURLs)
	if newTotal > r.maxMediaPerProduct {
		return nil, domainerrors.NewValidation(fmt.Sprintf(
			"adding %d URLs would bring product to %d total media, exceeding cap of %d",
			len(imageURLs)+len(videoURLs), newTotal, r.maxMediaPerProduct))
	}

	p.UpdatedAt = time.Now().UTC()
	m.ImageURLs = append(m.ImageURLs, imageURLs...)
	m.VideoURLs = append(m.VideoURLs, videoURLs...)

	counts := r.mediaCounts[id]
	counts.ImageCount += len(imageURLs)
	counts.VideoCount += len(videoURLs)
	r.mediaCounts[id] = counts

	// Update thumbnail cache if this is the first image ever added.
	if _, hasThumbnail := r.thumbnails[id]; !hasThumbnail && len(imageURLs) > 0 {
		r.thumbnails[id] = imageURLs[0]
	}

	// Build the response from the post-mutation state — all still under the same write lock.
	imgs := make([]string, len(m.ImageURLs))
	copy(imgs, m.ImageURLs)
	vids := make([]string, len(m.VideoURLs))
	copy(vids, m.VideoURLs)

	return &DetailResponse{
		ID:        p.ID,
		Name:      p.Name,
		SKU:       p.SKU,
		ImageURLs: imgs,
		VideoURLs: vids,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}, nil
}

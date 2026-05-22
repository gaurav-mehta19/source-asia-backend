package product

import (
	"context"
	"errors"
	"testing"
	"time"

	domainerrors "github.com/source-asia/backend/internal/shared/errors"
)

func newProduct(id, name, sku string) *Product {
	now := time.Now().UTC()
	return &Product{ID: id, Name: name, SKU: sku, CreatedAt: now, UpdatedAt: now}
}

// TestCreate_Then_GetByID_ReturnsAllMedia covers the basic round trip.
func TestCreate_Then_GetByID_ReturnsAllMedia(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	p := newProduct("p1", "Widget", "SKU-1")
	m := &Media{
		ImageURLs: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
		VideoURLs: []string{"https://cdn.example.com/c.mp4"},
	}
	if err := repo.Create(ctx, p, m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "p1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.ImageURLs) != 2 || len(got.VideoURLs) != 1 {
		t.Fatalf("expected 2 images and 1 video, got %d/%d", len(got.ImageURLs), len(got.VideoURLs))
	}
}

// TestGetByID_CopiesSlices verifies the defensive copy — mutating the returned slice must not affect storage.
func TestGetByID_CopiesSlices(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	p := newProduct("p1", "Widget", "SKU-1")
	m := &Media{ImageURLs: []string{"https://cdn.example.com/a.jpg"}}
	_ = repo.Create(ctx, p, m)

	first, _ := repo.GetByID(ctx, "p1")
	first.ImageURLs[0] = "MUTATED"

	second, _ := repo.GetByID(ctx, "p1")
	if second.ImageURLs[0] == "MUTATED" {
		t.Fatal("returned slice must be a copy — mutation leaked into storage")
	}
}

// TestCreate_DuplicateSKU_ReturnsConflict covers the 409 path.
func TestCreate_DuplicateSKU_ReturnsConflict(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	_ = repo.Create(ctx, newProduct("p1", "A", "DUPE"), &Media{})
	err := repo.Create(ctx, newProduct("p2", "B", "DUPE"), &Media{})
	if !errors.Is(err, domainerrors.ErrSKUConflict) {
		t.Fatalf("expected ErrSKUConflict, got %v", err)
	}
}

// TestGetByID_NotFound covers the 404 path.
func TestGetByID_NotFound(t *testing.T) {
	repo := NewRepository(200)
	_, err := repo.GetByID(context.Background(), "missing")
	if !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestList_DoesNotReturnURLArrays is the assignment's performance rule:
// the list payload must not contain full URL slices.
func TestList_DoesNotReturnURLArrays(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	imgs := make([]string, 10)
	for i := range imgs {
		imgs[i] = "https://cdn.example.com/" + itoa(i) + ".jpg"
	}
	_ = repo.Create(ctx, newProduct("p1", "Widget", "SKU-1"), &Media{ImageURLs: imgs})

	items, _, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.ImageCount != 10 {
		t.Fatalf("expected image_count=10, got %d", item.ImageCount)
	}
	if item.ThumbnailURL != imgs[0] {
		t.Fatalf("expected thumbnail %q, got %q", imgs[0], item.ThumbnailURL)
	}
	// ListItem deliberately has no ImageURLs / VideoURLs fields.
}

// TestList_Pagination verifies limit/offset semantics and total reporting.
func TestList_Pagination(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = repo.Create(ctx, newProduct("p"+itoa(i), "n"+itoa(i), "sku"+itoa(i)), &Media{})
	}

	items, total, _ := repo.List(ctx, 2, 0)
	if total != 5 || len(items) != 2 {
		t.Fatalf("page 1: expected 2 items / total 5, got %d / %d", len(items), total)
	}
	if items[0].ID != "p0" || items[1].ID != "p1" {
		t.Fatalf("page 1 ordering wrong: %s, %s", items[0].ID, items[1].ID)
	}

	items, _, _ = repo.List(ctx, 2, 4)
	if len(items) != 1 || items[0].ID != "p4" {
		t.Fatalf("page 3: expected 1 item p4, got %d items", len(items))
	}

	items, _, _ = repo.List(ctx, 10, 100)
	if len(items) != 0 {
		t.Fatalf("offset past end: expected empty page, got %d items", len(items))
	}
}

// TestAddMedia_UpdatesUpdatedAt guards against regression of Bug 2:
// UpdatedAt must advance when media is appended.
func TestAddMedia_UpdatesUpdatedAt(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	p := newProduct("p1", "Widget", "SKU-1")
	createdAt := p.CreatedAt
	_ = repo.Create(ctx, p, &Media{})

	// Sleep just enough that the monotonic clock advances measurably.
	time.Sleep(2 * time.Millisecond)

	got, err := repo.AddMedia(ctx, "p1", []string{"https://cdn.example.com/new.jpg"}, nil)
	if err != nil {
		t.Fatalf("AddMedia: %v", err)
	}
	if !got.UpdatedAt.After(createdAt) {
		t.Fatalf("UpdatedAt did not advance: created_at=%v updated_at=%v", createdAt, got.UpdatedAt)
	}
}

// TestAddMedia_AppendsAndUpdatesCounts verifies both the full slices and the denormalised counts advance.
func TestAddMedia_AppendsAndUpdatesCounts(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	_ = repo.Create(ctx, newProduct("p1", "Widget", "SKU-1"), &Media{
		ImageURLs: []string{"https://cdn.example.com/a.jpg"},
	})

	detail, err := repo.AddMedia(ctx, "p1",
		[]string{"https://cdn.example.com/b.jpg", "https://cdn.example.com/c.jpg"},
		[]string{"https://cdn.example.com/v.mp4"},
	)
	if err != nil {
		t.Fatalf("AddMedia: %v", err)
	}
	if len(detail.ImageURLs) != 3 || len(detail.VideoURLs) != 1 {
		t.Fatalf("detail: expected 3 images, 1 video, got %d/%d", len(detail.ImageURLs), len(detail.VideoURLs))
	}

	items, _, _ := repo.List(ctx, 10, 0)
	if items[0].ImageCount != 3 || items[0].VideoCount != 1 {
		t.Fatalf("list counts not updated: %+v", items[0])
	}
}

// TestAddMedia_ThumbnailSetWhenFirstImageAddedLater verifies the thumbnail cache is populated lazily.
func TestAddMedia_ThumbnailSetWhenFirstImageAddedLater(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()

	_ = repo.Create(ctx, newProduct("p1", "Widget", "SKU-1"), &Media{})

	items, _, _ := repo.List(ctx, 10, 0)
	if items[0].ThumbnailURL != "" {
		t.Fatalf("expected empty thumbnail before any images, got %q", items[0].ThumbnailURL)
	}

	if _, err := repo.AddMedia(ctx, "p1", []string{"https://cdn.example.com/first.jpg"}, nil); err != nil {
		t.Fatalf("AddMedia: %v", err)
	}

	items, _, _ = repo.List(ctx, 10, 0)
	if items[0].ThumbnailURL != "https://cdn.example.com/first.jpg" {
		t.Fatalf("thumbnail not populated after first image add, got %q", items[0].ThumbnailURL)
	}
}

// TestAddMedia_NotFound covers the 404 path.
func TestAddMedia_NotFound(t *testing.T) {
	repo := NewRepository(200)
	_, err := repo.AddMedia(context.Background(), "missing", []string{"https://cdn.example.com/a.jpg"}, nil)
	if !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestCreate_RejectsWhenTotalMediaExceedsCap covers the new per-product cap on Create.
func TestCreate_RejectsWhenTotalMediaExceedsCap(t *testing.T) {
	repo := NewRepository(3) // tiny cap to force the boundary
	ctx := context.Background()

	err := repo.Create(ctx, newProduct("p1", "W", "S1"), &Media{
		ImageURLs: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
		VideoURLs: []string{"https://cdn.example.com/c.mp4", "https://cdn.example.com/d.mp4"},
	})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for exceeding per-product cap, got %v", err)
	}
}

// TestAddMedia_RejectsWhenTotalWouldExceedCap covers the new cap on AddMedia.
// The check must run inside the same write lock as the append to be race-free.
func TestAddMedia_RejectsWhenTotalWouldExceedCap(t *testing.T) {
	repo := NewRepository(3)
	ctx := context.Background()

	_ = repo.Create(ctx, newProduct("p1", "W", "S1"), &Media{
		ImageURLs: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
	})

	// Already 2 of 3. Adding 2 more (total = 4) must fail.
	_, err := repo.AddMedia(ctx, "p1",
		[]string{"https://cdn.example.com/c.jpg"},
		[]string{"https://cdn.example.com/d.mp4"},
	)
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for exceeding cap, got %v", err)
	}

	// Adding exactly 1 more (total = 3) must succeed.
	detail, err := repo.AddMedia(ctx, "p1", []string{"https://cdn.example.com/c.jpg"}, nil)
	if err != nil {
		t.Fatalf("expected accept at exact cap, got %v", err)
	}
	if len(detail.ImageURLs) != 3 {
		t.Fatalf("expected 3 images after add at cap, got %d", len(detail.ImageURLs))
	}
}

// TestAddMedia_ReturnsDetailAtomically asserts the response reflects the state produced
// by this very call — no separate GetByID round trip needed (closes the old TOCTOU).
func TestAddMedia_ReturnsDetailAtomically(t *testing.T) {
	repo := NewRepository(200)
	ctx := context.Background()
	_ = repo.Create(ctx, newProduct("p1", "W", "S1"), &Media{})

	detail, err := repo.AddMedia(ctx, "p1",
		[]string{"https://cdn.example.com/x.jpg"},
		[]string{"https://cdn.example.com/y.mp4"},
	)
	if err != nil {
		t.Fatalf("AddMedia: %v", err)
	}
	if detail.ID != "p1" || len(detail.ImageURLs) != 1 || len(detail.VideoURLs) != 1 {
		t.Fatalf("unexpected detail returned by AddMedia: %+v", detail)
	}
	// And the returned slice is a copy — mutating it must not leak.
	detail.ImageURLs[0] = "MUTATED"
	again, _ := repo.GetByID(ctx, "p1")
	if again.ImageURLs[0] == "MUTATED" {
		t.Fatal("AddMedia returned a shared slice — mutation leaked")
	}
}

// itoa avoids pulling strconv for a single-purpose helper.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	return string(buf)
}

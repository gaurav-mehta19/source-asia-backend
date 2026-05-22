// Package product implements the product catalog API.
package product

import "time"

// Product holds the core fields of a product (stored in the primary map).
type Product struct {
	ID        string
	Name      string
	SKU       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Media holds the full URL slices for a product (stored in a separate map).
// The list endpoint never reads this struct — it uses the denormalised counts
// and thumbnail caches instead (see repository.go).
type Media struct {
	ImageURLs []string
	VideoURLs []string
}

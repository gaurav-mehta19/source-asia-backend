package product

import "time"

// CreateRequest is the JSON body for POST /products.
type CreateRequest struct {
	Name      string   `json:"name"`
	SKU       string   `json:"sku"`
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

// AddMediaRequest is the JSON body for POST /products/{id}/media.
type AddMediaRequest struct {
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

// ListItem is a single entry in the paginated list response.
// It deliberately excludes full URL slices — only counts and the cached thumbnail.
type ListItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SKU          string    `json:"sku"`
	ImageCount   int       `json:"image_count"`
	VideoCount   int       `json:"video_count"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Pagination carries pagination metadata.
type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// ListResponse is the response body for GET /products.
type ListResponse struct {
	Items      []ListItem `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// DetailResponse is the full product response for GET /products/{id}.
type DetailResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SKU       string    `json:"sku"`
	ImageURLs []string  `json:"image_urls"`
	VideoURLs []string  `json:"video_urls"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

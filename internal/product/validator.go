package product

import (
	"fmt"
	"strings"

	domainerrors "github.com/source-asia/backend/internal/shared/errors"
	urlvalidator "github.com/source-asia/backend/internal/shared/validator"
)

// validateURLList checks that urls is non-nil, within maxCount, and each URL passes URL validation.
func validateURLList(urls []string, field string, maxCount, maxLen int) error {
	if len(urls) > maxCount {
		return domainerrors.NewValidation(fmt.Sprintf("%s must contain at most %d URLs", field, maxCount))
	}
	for i, u := range urls {
		if err := urlvalidator.ValidateURL(u, maxLen); err != nil {
			return domainerrors.NewValidation(fmt.Sprintf("%s[%d]: %s", field, i, err.Error()))
		}
	}
	return nil
}

// ValidateCreateRequest validates the POST /products request body.
func ValidateCreateRequest(req *CreateRequest, maxURLs, maxURLLen int) error {
	req.Name = strings.TrimSpace(req.Name)
	req.SKU = strings.TrimSpace(req.SKU)

	if req.Name == "" {
		return domainerrors.NewValidation("name is required")
	}
	if len(req.Name) > 200 {
		return domainerrors.NewValidation("name must not exceed 200 characters")
	}
	if req.SKU == "" {
		return domainerrors.NewValidation("sku is required")
	}
	if len(req.SKU) > 100 {
		return domainerrors.NewValidation("sku must not exceed 100 characters")
	}
	if err := validateURLList(req.ImageURLs, "image_urls", maxURLs, maxURLLen); err != nil {
		return err
	}
	if err := validateURLList(req.VideoURLs, "video_urls", maxURLs, maxURLLen); err != nil {
		return err
	}
	return nil
}

// ValidateAddMediaRequest validates the POST /products/{id}/media request body.
func ValidateAddMediaRequest(req *AddMediaRequest, maxURLs, maxURLLen int) error {
	if len(req.ImageURLs) == 0 && len(req.VideoURLs) == 0 {
		return domainerrors.NewValidation("at least one of image_urls or video_urls must be non-empty")
	}
	if err := validateURLList(req.ImageURLs, "image_urls", maxURLs, maxURLLen); err != nil {
		return err
	}
	if err := validateURLList(req.VideoURLs, "video_urls", maxURLs, maxURLLen); err != nil {
		return err
	}
	return nil
}

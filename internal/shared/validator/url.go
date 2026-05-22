// Package validator provides reusable input validation helpers.
package validator

import (
	"fmt"
	"net/url"
)

// ValidateURL checks that rawURL is a valid absolute http/https URL within maxLen characters.
func ValidateURL(rawURL string, maxLen int) error {
	if len(rawURL) == 0 {
		return fmt.Errorf("url must not be empty")
	}
	if len(rawURL) > maxLen {
		return fmt.Errorf("url exceeds maximum length of %d characters", maxLen)
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("url is not valid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("url must have a host")
	}
	return nil
}

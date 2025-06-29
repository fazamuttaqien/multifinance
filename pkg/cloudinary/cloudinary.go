package cloudinary

import (
	"fmt"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/fazamuttaqien/multifinance/config"
)

// CloudinaryConfig holds Cloudinary configuration
type CloudinaryConfig struct {
	CloudName string
	APIKey    string
	APISecret string
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	URL   string `json:"url"`
	Error error  `json:"error,omitempty"`
	Type  string `json:"type"`
}

// UploadResponse represents the API response
type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// InitCloudinary creates a new Cloudinary service
func InitCloudinary(cfg *config.Config) (*cloudinary.Cloudinary, error) {
	cld, err := cloudinary.NewFromParams(
		cfg.CLOUDINARY_CLOUD,
		cfg.CLOUDINARY_API_KEY,
		cfg.CLOUDINARY_API_SECRET,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}

	return cld, nil
}

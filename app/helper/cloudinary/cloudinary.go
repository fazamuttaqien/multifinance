package cloudinary

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
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

// CloudinaryService handles Cloudinary operations
type CloudinaryService struct {
	client *cloudinary.Cloudinary
}

// NewCloudinaryService creates a new Cloudinary service
func NewCloudinaryService(config CloudinaryConfig) (*CloudinaryService, error) {
	cld, err := cloudinary.NewFromParams(config.CloudName, config.APIKey, config.APISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}

	return &CloudinaryService{
		client: cld,
	}, nil
}

// UploadImage uploads a single image to Cloudinary
func (cs *CloudinaryService) UploadImage(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Upload to Cloudinary
	uploadResult, err := cs.client.Upload.Upload(ctx, src, uploader.UploadParams{
		Folder:    folder,
		PublicID:  generatePublicID(file.Filename),
		Overwrite: func(b bool) *bool { return &b }(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}

	return uploadResult.SecureURL, nil
}

// generatePublicID generates a unique public ID for the uploaded file
func generatePublicID(filename string) string {
	// You can implement your own logic here
	// For simplicity, we'll use the filename without extension
	return filename[:len(filename)-len(filepath.Ext(filename))] + "_" + fmt.Sprintf("%d", time.Now().Unix())
}

package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type cloudinaryService struct {
	client *cloudinary.Cloudinary
}

// UploadImage implements CloudinaryService.
func (c *cloudinaryService) UploadImage(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Upload to Cloudinary
	uploadResult, err := c.client.Upload.Upload(ctx, src, uploader.UploadParams{
		Folder:    folder,
		PublicID:  generatePublicID(file.Filename),
		Overwrite: func(b bool) *bool { return &b }(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}

	return uploadResult.SecureURL, nil
}

func NewCloudinaryService(client *cloudinary.Cloudinary) CloudinaryService {
	return &cloudinaryService{
		client: client,
	}
}

// generatePublicID generates a unique public ID for the uploaded file
func generatePublicID(filename string) string {
	// You can implement your own logic here
	// For simplicity, we'll use the filename without extension
	return filename[:len(filename)-len(filepath.Ext(filename))] + "_" + fmt.Sprintf("%d", time.Now().Unix())
}

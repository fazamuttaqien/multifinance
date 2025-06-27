package main

import (
	"slices"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
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
	Success       bool   `json:"success"`
	PhotoImageURL string `json:"photo_image_url,omitempty"`
	BackgroundURL string `json:"background_image_url,omitempty"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
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

// uploadImagesHandler handles concurrent upload of two images
func uploadImagesHandler(cs *CloudinaryService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse multipart form
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(UploadResponse{
				Success: false,
				Error:   "Failed to parse multipart form: " + err.Error(),
			})
		}

		// Get files from form
		photoFiles := form.File["photoImage"]
		backgroundFiles := form.File["backgroundImage"]

		// Validate that both files are provided
		if len(photoFiles) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(UploadResponse{
				Success: false,
				Error:   "photoImage file is required",
			})
		}

		if len(backgroundFiles) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(UploadResponse{
				Success: false,
				Error:   "backgroundImage file is required",
			})
		}

		photoFile := photoFiles[0]
		backgroundFile := backgroundFiles[0]

		// Validate file types (basic validation)
		if !isValidImageType(photoFile.Header.Get("Content-Type")) {
			return c.Status(fiber.StatusBadRequest).JSON(UploadResponse{
				Success: false,
				Error:   "photoImage must be a valid image file",
			})
		}

		if !isValidImageType(backgroundFile.Header.Get("Content-Type")) {
			return c.Status(fiber.StatusBadRequest).JSON(UploadResponse{
				Success: false,
				Error:   "backgroundImage must be a valid image file",
			})
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use WaitGroup for concurrent uploads
		var wg sync.WaitGroup
		resultChan := make(chan UploadResult, 2)

		// Upload photo image concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			url, err := cs.UploadImage(ctx, photoFile, "photos")
			resultChan <- UploadResult{
				URL:   url,
				Error: err,
				Type:  "photo",
			}
		}()

		// Upload background image concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			url, err := cs.UploadImage(ctx, backgroundFile, "backgrounds")
			resultChan <- UploadResult{
				URL:   url,
				Error: err,
				Type:  "background",
			}
		}()

		// Wait for all uploads to complete
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results
		var photoURL, backgroundURL string
		var uploadErrors []string

		for result := range resultChan {
			if result.Error != nil {
				uploadErrors = append(uploadErrors, fmt.Sprintf("%s upload failed: %v", result.Type, result.Error))
				continue
			}

			switch result.Type {
			case "photo":
				photoURL = result.URL
			case "background":
				backgroundURL = result.URL
			}
		}

		// Check if there were any errors
		if len(uploadErrors) > 0 {
			return c.Status(fiber.StatusInternalServerError).JSON(UploadResponse{
				Success: false,
				Error:   fmt.Sprintf("Upload errors: %v", uploadErrors),
			})
		}

		// Return successful response
		return c.JSON(UploadResponse{
			Success:       true,
			PhotoImageURL: photoURL,
			BackgroundURL: backgroundURL,
			Message:       "Images uploaded successfully",
		})
	}
}

// isValidImageType checks if the content type is a valid image type
func isValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
	}

	return slices.Contains(validTypes, contentType)
}

func main() {
	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024, // 10MB limit
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Initialize Cloudinary service
	// Replace with your actual Cloudinary credentials
	cloudinaryService, err := NewCloudinaryService(CloudinaryConfig{
		CloudName: "drnswvvpg",
		APIKey:    "431672144453183",
		APISecret: "WXm7ZVLYRoynxlagvqcz55hZZFk",
	})
	if err != nil {
		log.Fatal("Failed to initialize Cloudinary service:", err)
	}

	// Routes
	app.Post("/upload-images", uploadImagesHandler(cloudinaryService))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Server is running",
		})
	})

	// Start server
	log.Println("Server starting on :3000")
	log.Fatal(app.Listen(":3000"))
}

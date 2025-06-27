package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/cloudinary"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ProfileHandler struct {
	profileService    service.ProfileServices
	validate          *validator.Validate
	cloudinaryService *cloudinary.CloudinaryService
}

func NewProfileHandler(
	profileService service.ProfileServices,
	cloudinaryService *cloudinary.CloudinaryService,
) *ProfileHandler {
	return &ProfileHandler{
		profileService:    profileService,
		validate:          validator.New(validator.WithRequiredStructEnabled()),
		cloudinaryService: cloudinaryService,
	}
}

func (h *ProfileHandler) Register(c *fiber.Ctx) error {
	var req dto.Register

	// Parse multipart form
	if err := c.BodyParser(&req); err != nil {
		return common.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Get files from form
	ktpPhotoFile, err := c.FormFile("ktp_photo")
	if err != nil {
		return common.ErrorResponse(c, fiber.StatusBadRequest, "KTP photo is a required form field")
	}

	selfiePhotoFile, err := c.FormFile("selfie_photo")
	if err != nil {
		return common.ErrorResponse(c, fiber.StatusBadRequest, "Selfie photo is a required form field")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use WaitGroup for concurrent uploads
	var wg sync.WaitGroup
	resultChan := make(chan cloudinary.UploadResult, 2)

	// Upload ktp image concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		url, err := h.cloudinaryService.UploadImage(ctx, ktpPhotoFile, "photos")
		resultChan <- cloudinary.UploadResult{
			URL:   url,
			Error: err,
			Type:  "ktp",
		}
	}()

	// Upload selfie image concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		url, err := h.cloudinaryService.UploadImage(ctx, selfiePhotoFile, "photos")
		resultChan <- cloudinary.UploadResult{
			URL:   url,
			Error: err,
			Type:  "selfie",
		}
	}()

	// Wait for all uploads to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var ktpUrl, selfieUrl string
	var uploadErrors []string

	for result := range resultChan {
		if result.Error != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s upload failed: %v", result.Type, result.Error))
			continue
		}

		switch result.Type {
		case "ktp":
			ktpUrl = result.URL
		case "selfie":
			selfieUrl = result.URL
		}
	}

	// Check if there were any errors
	if len(uploadErrors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(cloudinary.UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("Upload errors: %v", uploadErrors),
		})
	}

	if err := h.validate.Struct(&req); err != nil {
		return common.ErrorResponse(c, fiber.StatusBadRequest, err.Error())
	}

	dtoRegister := dto.RegisterToEntity(req, ktpUrl, selfieUrl)
	newCustomer, err := h.profileService.Register(c.Context(), dtoRegister)
	if err != nil {
		if err.Error() == "nik already registered" {
			return common.ErrorResponse(c, fiber.StatusConflict, err.Error())
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrorResponse(c, fiber.StatusConflict, "NIK already registered")
		}
		return common.ErrorResponse(c, fiber.StatusInternalServerError, "Could not process registration")
	}

	return common.SuccessResponse(c, fiber.StatusCreated, newCustomer)
}

func (h *ProfileHandler) GetMyProfile(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	customer, err := h.profileService.GetProfile(c.Context(), customerID)
	if err != nil {
		// ... (error handling seperti sebelumnya)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(customer)
}

func (h *ProfileHandler) UpdateMyProfile(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req dto.Update

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request"})
	}

	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	dtoUpdate := dto.UpdateToEntity(req)
	if err := h.profileService.UpdateProfile(c.Context(), customerID, dtoUpdate); err != nil {
		// ... (error handling)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Profile updated successfully"})
}

func (h *ProfileHandler) GetMyLimits(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	limits, err := h.profileService.GetMyLimits(c.Context(), customerID)
	if err != nil {
		// ... (error handling)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(limits)
}

func (h *ProfileHandler) GetMyTransactions(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse query params untuk paginasi
	params := domain.Params{
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
	}

	response, err := h.profileService.GetMyTransactions(c.Context(), customerID, params)
	if err != nil {
		// ... (error handling)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

func (h *ProfileHandler) CreateTransaction(c *fiber.Ctx) error {
	// 1. Parse request body (JSON)
	var req dto.Transaction
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request body"})
	}

	// 2. Validasi struct request
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Validation failed", "details": err.Error()})
	}

	// 3. Panggil service
	createdTx, err := h.profileService.CreateTransaction(c.Context(), req)
	if err != nil {
		// Mapping error
		switch {
		case errors.Is(err, common.ErrCustomerNotFound), errors.Is(err, common.ErrTenorNotFound):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, common.ErrInsufficientLimit), errors.Is(err, common.ErrLimitNotSet):
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"status": "rejected", "reason": err.Error()})
		default:
			log.Printf("Internal server error on CreateTransaction: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "An internal server error occurred"})
		}
	}

	// 4. Kirim response sukses
	return c.Status(fiber.StatusCreated).JSON(createdTx)
}

// getCustomerIDFromLocals adalah helper untuk mengambil ID customer dari context Fiber
func getCustomerIDFromLocals(c *fiber.Ctx) (uint64, error) {
	idVal := c.Locals("customerID")
	id, ok := idVal.(uint64)
	if !ok {
		return 0, errors.New("customerID not found or invalid in context")
	}
	return id, nil
}

package controllers

import (
	"errors"
	"log"
	"strconv"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/usecases"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Controller struct {
	usecase  usecases.Usecases
	validate *validator.Validate
}

func NewController(usecase usecases.Usecases) *Controller {
	return &Controller{
		usecase:  usecase,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (cc *Controller) RegisterCustomer(c *fiber.Ctx) error {
	var req dtos.CustomerRegister

	// Parse multipart form
	if err := c.BodyParser(&req); err != nil {
		return helper.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Get files from form
	ktpPhotoFile, err := c.FormFile("ktp_photo")
	if err != nil {
		return helper.ErrorResponse(c, fiber.StatusBadRequest, "KTP photo is a required form field")
	}
	req.KTPPhoto = ktpPhotoFile

	selfiePhotoFile, err := c.FormFile("selfie_photo")
	if err != nil {
		return helper.ErrorResponse(c, fiber.StatusBadRequest, "Selfie photo is a required form field")
	}
	req.SelfiePhoto = selfiePhotoFile

	if err := cc.validate.Struct(&req); err != nil {
		return helper.ErrorResponse(c, fiber.StatusBadRequest, err.Error())
	}

	newCustomer, err := cc.usecase.Register(c.Context(), &req)
	if err != nil {
		if err.Error() == "nik already registered" {
			return helper.ErrorResponse(c, fiber.StatusConflict, err.Error())
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.ErrorResponse(c, fiber.StatusConflict, "NIK already registered")
		}
		return helper.ErrorResponse(c, fiber.StatusInternalServerError, "Could not process registration")
	}

	return helper.SuccessResponse(c, fiber.StatusCreated, newCustomer)
}

func (cc *Controller) GetLimit(c *fiber.Ctx) error {
	// 1. Parse parameter dari URL
	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid customer ID format",
		})
	}

	tenorMonths, err := strconv.ParseUint(c.Params("tenorMonths"), 10, 8)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid tenor months format",
		})
	}

	// 2. Calling service
	limitDetails, err := cc.usecase.CalculateLimit(c.Context(), customerID, uint8(tenorMonths))
	if err != nil {
		switch {
		case errors.Is(err, helper.ErrCustomerNotFound), errors.Is(err, helper.ErrTenorNotFound), errors.Is(err, helper.ErrLimitNotSet):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		default:
			// Log error asli untuk debugging
			log.Printf("Internal server error on GetLimit: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "An Internal Server Error Occured",
			})
		}
	}

	return c.Status(fiber.StatusOK).JSON(limitDetails)
}

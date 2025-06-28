package handler

import (
	"errors"
	"log"
	"strconv"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	adminService service.AdminServices
	validate     *validator.Validate
}

func (h *AdminHandler) ListCustomers(c *fiber.Ctx) error {
	params := domain.Params{
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
	}

	res, err := h.adminService.ListCustomers(c.Context(), params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(res)
}

func (h *AdminHandler) GetCustomerByID(c *fiber.Ctx) error {
	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid customer ID",
		})
	}

	customer, err := h.adminService.GetCustomerByID(c.Context(), customerID)
	if err != nil {
		if errors.Is(err, common.ErrCustomerNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(customer)
}

func (h *AdminHandler) VerifyCustomer(c *fiber.Ctx) error {
	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid customer ID"})
	}

	var req dto.VerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request body"})
	}

	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.adminService.VerifyCustomer(c.Context(), customerID, req); err != nil {
		if errors.Is(err, common.ErrCustomerNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Customer verification status updated"})
}

func (h *AdminHandler) SetLimits(c *fiber.Ctx) error {
	// 1. Parse Customer ID dari URL
	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid customer ID format",
		})
	}

	// 2. Parse request body (JSON)
	var req dto.SetLimits
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse request body",
		})
	}

	// 3. Validasi struct request
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"defails": err.Error(),
		})
	}

	// 4. Panggil service
	if err := h.adminService.SetLimits(c.Context(), customerID, req); err != nil {
		switch {
		case errors.Is(err, common.ErrCustomerNotFound), errors.Is(err, common.ErrTenorNotFound):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, common.ErrInvalidLimitAmount):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		default:
			log.Printf("Internal server error on SetLimits: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "An internal server error occurred"})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Customer limits updated successfully",
	})
}

func NewAdminHandler(adminService service.AdminServices) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
		validate:     validator.New(validator.WithRequiredStructEnabled()),
	}
}

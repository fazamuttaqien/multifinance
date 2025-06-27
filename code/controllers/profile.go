package controllers

import (
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/usecases"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type ProfileController struct {
	usecase   usecases.ProfileUsecases
	validator *validator.Validate
}

func NewProfileController(u usecases.PartnerUsecases) *PartnerController {
	return &PartnerController{
		usecase:   u,
		validator: validator.New(validator.WithRequiredStructEnabled()),
	}
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

func (h *ProfileController) GetMyProfile(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	customer, err := h.usecase.GetProfile(c.Context(), customerID)
	if err != nil {
		// ... (error handling seperti sebelumnya)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(customer)
}

func (h *ProfileController) UpdateMyProfile(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req dtos.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request"})
	}

	if err := h.validator.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.usecase.UpdateProfile(c.Context(), customerID, req); err != nil {
		// ... (error handling)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Profile updated successfully"})
}

func (h *ProfileController) GetMyLimits(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	limits, err := h.usecase.GetMyLimits(c.Context(), customerID)
	if err != nil {
		// ... (error handling)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(limits)
}

func (h *ProfileController) GetMyTransactions(c *fiber.Ctx) error {
	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse query params untuk paginasi
	params := dtos.PaginationParams{
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
	}

	response, err := h.usecase.GetMyTransactions(c.Context(), customerID, params)
	if err != nil {
		// ... (error handling)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

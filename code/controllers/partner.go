package controllers

import (
	"errors"
	"log"
	"net/http"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/usecases"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type PartnerController struct {
	usecase   usecases.PartnerUsecases
	validator *validator.Validate
}

func NewPartnerController(u usecases.PartnerUsecases) *PartnerController {
	return &PartnerController{
		usecase:   u,
		validator: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (p *PartnerController) CheckLimit(c *fiber.Ctx) error {
	var req dtos.CheckLimitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request body"})
	}

	if err := p.validator.Struct(req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res, err := p.usecase.CheckLimit(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, helper.ErrCustomerNotFound) || errors.Is(err, helper.ErrTenorNotFound) || errors.Is(err, helper.ErrLimitNotSet):
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		default:
			log.Printf("Internal error on CheckLimit: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
		}
	}

	if res.Status == "rejected" {
		return c.Status(http.StatusUnprocessableEntity).JSON(res)
	}

	return c.Status(http.StatusOK).JSON(res)
}

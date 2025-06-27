package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type PartnerHandler struct {
	partnerService service.PartnerServices
	validator      *validator.Validate
}

func NewPartnerHandler(partnerService service.PartnerServices) *PartnerHandler {
	return &PartnerHandler{
		partnerService: partnerService,
		validator:      validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (p *PartnerHandler) CheckLimit(c *fiber.Ctx) error {
	var req dto.CheckLimitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse request body"})
	}

	if err := p.validator.Struct(req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res, err := p.partnerService.CheckLimit(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrCustomerNotFound) || errors.Is(err, common.ErrTenorNotFound) || errors.Is(err, common.ErrLimitNotSet):
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

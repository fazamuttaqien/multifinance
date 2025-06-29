package dto

import (
	"mime/multipart"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
)

type CreateProfileRequest struct {
	NIK         string                `form:"nik" validate:"required,len=16,numeric"`
	FullName    string                `form:"full_name" validate:"required"`
	LegalName   string                `form:"legal_name" validate:"required"`
	BirthPlace  string                `form:"birth_place" validate:"required"`
	BirthDate   string                `form:"birth_date" validate:"required,datetime=2006-01-02"`
	Salary      float64               `form:"salary" validate:"required,gt=0"`
	KtpPhoto    *multipart.FileHeader `form:"ktp_photo" validate:"required"`
	SelfiePhoto *multipart.FileHeader `form:"selfie_photo" validate:"required"`
}

type UpdateProfileRequest struct {
	FullName string  `json:"full_name" validate:"required"`
	Salary   float64 `json:"salary" validate:"required,gt=0"`
}

type CreateTransactionRequest struct {
	CustomerNIK string  `json:"customer_nik" validate:"required,len=16,numeric"`
	TenorMonths uint8   `json:"tenor_months" validate:"required,gt=0"`
	AssetName   string  `json:"asset_name" validate:"required"`
	OTRAmount   float64 `json:"otr_amount" validate:"required,gt=0"`
	AdminFee    float64 `json:"admin_fee" validate:"required,gte=0"`
}

type LimitItemRequest struct {
	TenorMonths uint8   `json:"tenor_months" validate:"required,gt=0"`
	LimitAmount float64 `json:"limit_amount" validate:"required,gte=0"`
}

type SetLimits struct {
	Limits []LimitItemRequest `json:"limits" validate:"required,min=1,dive"`
}

type CheckLimitRequest struct {
	CustomerNIK       string  `json:"customer_nik" validate:"required,len=16,numeric"`
	TenorMonths       uint8   `json:"tenor_months" validate:"required,gt=0"`
	TransactionAmount float64 `json:"transaction_amount" validate:"required,gt=0"`
}

type VerificationRequest struct {
	Status domain.VerificationStatus `json:"status" validate:"required,oneof=VERIFIED REJECTED"`
	Reason string                    `json:"reason,omitempty"`
}

// --- Mapping --- //

func RegisterToEntity(req CreateProfileRequest, ktpUrl, selfieUrl string) *domain.Customer {
	birthDate, _ := time.Parse("2006-01-02", req.BirthDate)
	return &domain.Customer{
		NIK:                req.NIK,
		FullName:           req.FullName,
		LegalName:          req.LegalName,
		BirthPlace:         req.BirthPlace,
		BirthDate:          birthDate,
		Salary:             req.Salary,
		KtpUrl:             ktpUrl,
		SelfieUrl:          selfieUrl,
		VerificationStatus: domain.VerificationPending,
	}
}

func UpdateToEntity(req UpdateProfileRequest) domain.Customer {
	return domain.Customer{
		FullName: req.FullName,
		Salary:   req.Salary,
	}
}

package dtos

import "mime/multipart"

type CustomerRegister struct {
	NIK         string                `form:"nik" validate:"required,len=16,numeric"`
	FullName    string                `form:"full_name" validate:"required"`
	LegalName   string                `form:"legal_name" validate:"required"`
	BirthPlace  string                `form:"birth_place" validate:"required"`
	BirthDate   string                `form:"birth_date" validate:"required,datetime=2006-01-02"`
	Salary      float64               `form:"salary" validate:"required,gt=0"`
	KTPPhoto    *multipart.FileHeader `form:"ktp_photo" validate:"required"`
	SelfiePhoto *multipart.FileHeader `form:"selfie_photo" validate:"required"`
}

type LimitItem struct {
	TenorMonths uint8   `json:"tenor_months" validate:"required,gt=0"`
	LimitAmount float64 `json:"limit_amount" validate:"required,gte=0"`
}

type SetLimitsRequest struct {
	Limits []LimitItem `json:"limits" validate:"required,min=1,dive"`
}

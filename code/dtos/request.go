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

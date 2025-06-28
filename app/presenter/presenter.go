package presenter

import (
	"github.com/fazamuttaqien/multifinance/handler"
	"github.com/fazamuttaqien/multifinance/repository"
	"github.com/fazamuttaqien/multifinance/service"

	"github.com/cloudinary/cloudinary-go/v2"
	"gorm.io/gorm"
)

type Presenter struct {
	AdminPresenter   *handler.AdminHandler
	PartnerPresenter *handler.PartnerHandler
	ProfilePresenter *handler.ProfileHandler
}

func NewPresenter(db *gorm.DB, cld *cloudinary.Cloudinary) Presenter {
	customerRepository := repository.NewCustomerRepository(db)
	limitRepository := repository.NewLimitRepository(db)
	tenorRepository := repository.NewTenorRepository(db)
	transactionRepository := repository.NewTransactionRepository(db)

	adminService := service.NewAdminService(db, customerRepository)
	partnerService := service.NewPartnerService(db, customerRepository, tenorRepository, limitRepository, transactionRepository)
	profileService := service.NewProfileService(db, customerRepository, limitRepository, tenorRepository, transactionRepository)
	cloudinaryService := service.NewCloudinaryService(cld)

	adminHandler := handler.NewAdminHandler(adminService)
	partnerHandler := handler.NewPartnerHandler(partnerService)
	profileHandler := handler.NewProfileHandler(profileService, cloudinaryService)

	return Presenter{
		AdminPresenter:   adminHandler,
		PartnerPresenter: partnerHandler,
		ProfilePresenter: profileHandler,
	}
}

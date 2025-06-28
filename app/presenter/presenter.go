package presenter

import (
	"github.com/fazamuttaqien/multifinance/handler"
	"github.com/fazamuttaqien/multifinance/repository"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/fazamuttaqien/multifinance/telemetry"

	"github.com/cloudinary/cloudinary-go/v2"
	"gorm.io/gorm"
)

type Presenter struct {
	AdminPresenter   *handler.AdminHandler
	PartnerPresenter *handler.PartnerHandler
	ProfilePresenter *handler.ProfileHandler
}

func NewPresenter(
	db *gorm.DB,
	cld *cloudinary.Cloudinary,
	tel *telemetry.OpenTelemetry,
) Presenter {
	// Repository
	customerRepositoryMeter := tel.MeterProvider.Meter("customer-repository-meter")
	customerRepositoryTracer := tel.TracerProvider.Tracer("customer-repository-tracer")
	customerRepository := repository.NewCustomerRepository(
		db,
		customerRepositoryMeter,
		customerRepositoryTracer,
		tel.Log,
	)

	limitRepositoryMeter := tel.MeterProvider.Meter("limit-repository-meter")
	limitRepositoryTracer := tel.TracerProvider.Tracer("limit-repository-tracer")
	limitRepository := repository.NewLimitRepository(
		db,
		limitRepositoryMeter,
		limitRepositoryTracer,
		tel.Log,
	)

	tenorRepositoryMeter := tel.MeterProvider.Meter("limit-repository-meter")
	tenorRepositoryTracer := tel.TracerProvider.Tracer("limit-repository-tracer")
	tenorRepository := repository.NewTenorRepository(
		db,
		tenorRepositoryMeter,
		tenorRepositoryTracer,
		tel.Log,
	)

	transactionRepositoryMeter := tel.MeterProvider.Meter("limit-repository-meter")
	transactionRepositoryTracer := tel.TracerProvider.Tracer("limit-repository-tracer")
	transactionRepository := repository.NewTransactionRepository(
		db,
		transactionRepositoryMeter,
		transactionRepositoryTracer,
		tel.Log,
	)

	// Service
	adminServiceMeter := tel.MeterProvider.Meter("admin-service-meter")
	adminServiceTracer := tel.TracerProvider.Tracer("admin-service-trace")
	adminService := service.NewAdminService(
		db,
		customerRepository,
		adminServiceMeter,
		adminServiceTracer,
		tel.Log,
	)

	partnerServiceMeter := tel.MeterProvider.Meter("partner-service-meter")
	partnerServiceTracer := tel.TracerProvider.Tracer("partner-service-trace")
	partnerService := service.NewPartnerService(
		db,
		customerRepository,
		tenorRepository,
		limitRepository,
		transactionRepository,
		partnerServiceMeter,
		partnerServiceTracer,
		tel.Log,
	)

	profileServiceMeter := tel.MeterProvider.Meter("profile-service-meter")
	profileServiceTracer := tel.TracerProvider.Tracer("profile-service-trace")
	profileService := service.NewProfileService(
		db,
		customerRepository,
		limitRepository,
		tenorRepository,
		transactionRepository,
		profileServiceMeter,
		profileServiceTracer,
		tel.Log,
	)

	cloudinaryService := service.NewCloudinaryService(cld)

	// Handler
	adminHandler := handler.NewAdminHandler(adminService)
	partnerHandler := handler.NewPartnerHandler(partnerService)
	profileHandler := handler.NewProfileHandler(profileService, cloudinaryService)

	return Presenter{
		AdminPresenter:   adminHandler,
		PartnerPresenter: partnerHandler,
		ProfilePresenter: profileHandler,
	}
}

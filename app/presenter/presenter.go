package presenter

import (
	adminhandler "github.com/fazamuttaqien/multifinance/internal/handler/admin"
	partnerhandler "github.com/fazamuttaqien/multifinance/internal/handler/partner"
	profilehandler "github.com/fazamuttaqien/multifinance/internal/handler/profile"
	customerrepo "github.com/fazamuttaqien/multifinance/internal/repository/customer"
	limitrepo "github.com/fazamuttaqien/multifinance/internal/repository/limit"
	tenorrepo "github.com/fazamuttaqien/multifinance/internal/repository/tenor"
	transactionrepo "github.com/fazamuttaqien/multifinance/internal/repository/transaction"
	adminsrv "github.com/fazamuttaqien/multifinance/internal/service/admin"
	cloudinarysrv "github.com/fazamuttaqien/multifinance/internal/service/cloudinary"
	partnersrv "github.com/fazamuttaqien/multifinance/internal/service/partner"
	profilesrv "github.com/fazamuttaqien/multifinance/internal/service/profile"

	"github.com/fazamuttaqien/multifinance/pkg/telemetry"

	"github.com/cloudinary/cloudinary-go/v2"
	"gorm.io/gorm"
)

type Presenter struct {
	AdminPresenter   *adminhandler.AdminHandler
	PartnerPresenter *partnerhandler.PartnerHandler
	ProfilePresenter *profilehandler.ProfileHandler
}

func NewPresenter(
	db *gorm.DB,
	cld *cloudinary.Cloudinary,
	tel *telemetry.OpenTelemetry,
) Presenter {
	// Repository
	customerRepositoryMeter := tel.MeterProvider.Meter("customer-repository-meter")
	customerRepositoryTracer := tel.TracerProvider.Tracer("customer-repository-tracer")
	customerRepository := customerrepo.NewCustomerRepository(
		db,
		customerRepositoryMeter,
		customerRepositoryTracer,
		tel.Log,
	)

	limitRepositoryMeter := tel.MeterProvider.Meter("limit-repository-meter")
	limitRepositoryTracer := tel.TracerProvider.Tracer("limit-repository-tracer")
	limitRepository := limitrepo.NewLimitRepository(
		db,
		limitRepositoryMeter,
		limitRepositoryTracer,
		tel.Log,
	)

	tenorRepositoryMeter := tel.MeterProvider.Meter("limit-repository-meter")
	tenorRepositoryTracer := tel.TracerProvider.Tracer("limit-repository-tracer")
	tenorRepository := tenorrepo.NewTenorRepository(
		db,
		tenorRepositoryMeter,
		tenorRepositoryTracer,
		tel.Log,
	)

	transactionRepositoryMeter := tel.MeterProvider.Meter("limit-repository-meter")
	transactionRepositoryTracer := tel.TracerProvider.Tracer("limit-repository-tracer")
	transactionRepository := transactionrepo.NewTransactionRepository(
		db,
		transactionRepositoryMeter,
		transactionRepositoryTracer,
		tel.Log,
	)

	// Service
	adminServiceMeter := tel.MeterProvider.Meter("admin-service-meter")
	adminServiceTracer := tel.TracerProvider.Tracer("admin-service-trace")
	adminService := adminsrv.NewAdminService(
		db,
		customerRepository,
		adminServiceMeter,
		adminServiceTracer,
		tel.Log,
	)

	partnerServiceMeter := tel.MeterProvider.Meter("partner-service-meter")
	partnerServiceTracer := tel.TracerProvider.Tracer("partner-service-trace")
	partnerService := partnersrv.NewPartnerService(
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
	profileService := profilesrv.NewProfileService(
		db,
		customerRepository,
		limitRepository,
		tenorRepository,
		transactionRepository,
		profileServiceMeter,
		profileServiceTracer,
		tel.Log,
	)

	cloudinaryService := cloudinarysrv.NewCloudinaryService(cld)

	// Handler
	adminHandlerMeter := tel.MeterProvider.Meter("admin-handler-meter")
	adminHandlerTracer := tel.TracerProvider.Tracer("admin-handler-trace")
	adminHandler := adminhandler.NewAdminHandler(
		adminService,
		adminHandlerMeter,
		adminHandlerTracer,
		tel.Log,
	)

	partnerHandlerMeter := tel.MeterProvider.Meter("partner-handler-meter")
	partnerHandlerTracer := tel.TracerProvider.Tracer("partner-handler-trace")
	partnerHandler := partnerhandler.NewPartnerHandler(
		partnerService,
		partnerHandlerMeter,
		partnerHandlerTracer,
		tel.Log,
	)

	profileHandlerMeter := tel.MeterProvider.Meter("profile-handler-meter")
	profileHandlerTracer := tel.TracerProvider.Tracer("profile-handler-trace")
	profileHandler := profilehandler.NewProfileHandler(
		profileService,
		cloudinaryService,
		profileHandlerMeter,
		profileHandlerTracer,
		tel.Log,
	)

	return Presenter{
		AdminPresenter:   adminHandler,
		PartnerPresenter: partnerHandler,
		ProfilePresenter: profileHandler,
	}
}

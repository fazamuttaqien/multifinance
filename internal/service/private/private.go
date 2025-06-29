package privatesrv

import (
	"context"
	"log"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	"github.com/fazamuttaqien/multifinance/internal/service"
	"github.com/fazamuttaqien/multifinance/pkg/common"
	"github.com/fazamuttaqien/multifinance/pkg/password"
	"github.com/golang-jwt/jwt/v5"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type privateService struct {
	db                 *gorm.DB
	customerRepository repository.CustomerRepository

	jwtSecret string

	meter             metric.Meter
	tracer            trace.Tracer
	log               *zap.Logger
	operationDuration metric.Float64Histogram
	operationCount    metric.Int64Counter
	errorCount        metric.Int64Counter
	profilesCreated   metric.Int64Counter
	profilesRetrieved metric.Int64Counter
	profilesUpdated   metric.Int64Counter
}

// Login implements service.PrivateService.
func (p *privateService) Login(ctx context.Context, data dto.LoginRequest) (*dto.LoginResponse, error) {
	cust, err := p.customerRepository.FindByNIK(ctx, data.NIK)
	if err != nil {
		return nil, err
	}
	log.Println("Hello")
	if cust == nil || !password.CheckPasswordHash(data.Password, cust.Password) {
		return nil, common.ErrInvalidCredentials
	}

	claims := &domain.JwtCustomClaims{
		UserID: cust.ID,
		Role:   cust.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)),
			Issuer:    "multifinance",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(p.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{Token: signedToken}, nil
}

func NewPrivateService(
	db *gorm.DB,
	jwtSecret string,
	customerRepository repository.CustomerRepository,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) service.PrivateService {
	operationDuration, _ := meter.Float64Histogram(
		"service.operation.duration",
		metric.WithDescription("Duration of service operations"),
		metric.WithUnit("ms"),
	)

	operationCount, _ := meter.Int64Counter(
		"service.operation.count",
		metric.WithDescription("Number of service operations"),
		metric.WithUnit("{operation}"),
	)

	errorCount, _ := meter.Int64Counter(
		"service.error.count",
		metric.WithDescription("Number of service errors"),
		metric.WithUnit("{error}"),
	)

	profilesCreated, _ := meter.Int64Counter(
		"service.profiles.created",
		metric.WithDescription("Number of profiles created"),
		metric.WithUnit("{profile}"),
	)

	profilesRetrieved, _ := meter.Int64Counter(
		"service.profiles.retrieved",
		metric.WithDescription("Number of profiles retrieved"),
		metric.WithUnit("{profile}"),
	)

	profilesUpdated, _ := meter.Int64Counter(
		"service.profiles.updated",
		metric.WithDescription("Number of profiles updated"),
		metric.WithUnit("{profile}"),
	)

	return &privateService{
		db:                 db,
		customerRepository: customerRepository,

		jwtSecret: jwtSecret,

		meter:             meter,
		tracer:            tracer,
		log:               log,
		operationDuration: operationDuration,
		operationCount:    operationCount,
		errorCount:        errorCount,
		profilesCreated:   profilesCreated,
		profilesRetrieved: profilesRetrieved,
		profilesUpdated:   profilesUpdated,
	}
}

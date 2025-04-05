// Package user provides user management and profile update services.
package user

import (
	"context"
	"io"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// S3Repository provides access to S3 storage.
//
//go:generate go tool mockery --name=S3Repository
type S3Repository interface {
	UploadAvatar(ctx context.Context, file io.Reader, fileName, mimeType string) error
	DeleteAvatar(ctx context.Context, fileName string) error
}

// Repository provides access to user data.
//
//go:generate go tool mockery --name=Repository
type Repository interface {
	repository.TxManager
	GetUserByID(ctx context.Context, id int) (user.PrivateProfile, error)
	UpdateUser(ctx context.Context, updateInfo dto.UpdateUserRequest) error
	UpdateAvatar(ctx context.Context, req dto.UpdateAvatarRequestDB) error
}

// Usecase contains business logic for user management.
type Usecase struct {
	cfg    Config
	tracer trace.Tracer
	repo   Repository
	s3     S3Repository
}

// NewUsecase creates and returns a new Handler instance with the provided dependencies.
//
// cfg - Configuration settings for the Handler.
//
// repo - Implementation of the Repository interface for accessing user data.
//
// s3 - Implementation of the S3Repository interface for accessing S3 storage.
func NewUsecase(cfg Config, repo Repository, s3 S3Repository) *Usecase {
	return &Usecase{
		cfg:    cfg,
		tracer: otel.GetTracerProvider().Tracer("UserUsecase"),
		repo:   repo,
		s3:     s3,
	}
}

// Package user contains HTTP handlers for user-related operations.
package user

import (
	"context"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// TokenService defines the interface for handling user JWT token operations.
type TokenService interface {
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
}

// Usecase defines the interface for user-related use case operations.
type Usecase interface {
	GetPrivateProfile(ctx context.Context, userID int) (user.PrivateProfile, error)
	GetPublicProfile(ctx context.Context, userID int) (user.PublicProfile, error)
	UpdateUser(ctx context.Context, req dto.UpdateUserRequest) error
	UpdateAvatar(ctx context.Context, req dto.UpdateAvatarRequest) error
}

var _ api.UsersHandler = (*Handler)(nil)

// Handler implements the api.UsersHandler interface and handles HTTP requests for user operations.
type Handler struct {
	cfg Config
	uc  Usecase
	ts  TokenService
}

// NewHandler creates and returns a new Handler instance with the provided dependencies.
//
// cfg - Configuration settings for the Handler.
//
// usecase - Implementation of the Usecase interface for business logic.
//
// tokenService - Implementation of the TokenService interface for handling tokens.
func NewHandler(
	cfg Config,
	usecase Usecase,
	tokenService TokenService,
) *Handler {
	return &Handler{
		cfg: cfg,
		uc:  usecase,
		ts:  tokenService,
	}
}

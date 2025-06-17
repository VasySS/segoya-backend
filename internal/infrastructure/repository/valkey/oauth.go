package valkey

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/google/uuid"
)

const (
	oauthPrefix = "oauthState:"
)

// NewOAuthState stores OAuth state and user id that is associated with it for later usage in a callback.
func (r *Repository) NewOAuthState(ctx context.Context, req dto.NewOAuthRequest) error {
	ctx, span := r.tracer.Start(ctx, "NewOAuthState")
	defer span.End()

	key := oauthPrefix + req.State
	cmd := r.valkey.B().Set().Key(key).Value(req.UserID.String()).Ex(req.StateTTL).Build()

	if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to create oauth state: %w", err)
	}

	return nil
}

// GetOAuthUserID returns user id associated with OAuth state.
func (r *Repository) GetOAuthUserID(ctx context.Context, state string) (uuid.UUID, error) {
	ctx, span := r.tracer.Start(ctx, "GetOAuthUserID")
	defer span.End()

	key := oauthPrefix + state
	cmd := r.valkey.B().Get().Key(key).Build()

	userID, err := r.valkey.Do(ctx, cmd).ToString()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to get oauth state: %w", err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to parse user's uuid from session: %w", err)
	}

	return userUUID, nil
}

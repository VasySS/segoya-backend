package valkey

import (
	"context"
	"fmt"
	"strconv"

	"github.com/VasySS/segoya-backend/internal/dto"
)

const (
	oauthPrefix = "oauthState:"
)

// NewOAuthState stores oauth state and user id that is associated with it for later checks in callback.
func (r *Repository) NewOAuthState(ctx context.Context, req dto.NewOAuthRequest) error {
	ctx, span := r.tracer.Start(ctx, "NewOAuthState")
	defer span.End()

	key := oauthPrefix + req.State
	cmd := r.valkey.B().Set().Key(key).Value(strconv.Itoa(req.UserID)).Ex(req.StateTTL).Build()

	if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to create oauth state: %w", err)
	}

	return nil
}

// GetOAuthUserID returns user id associated with oauth state.
func (r *Repository) GetOAuthUserID(ctx context.Context, state string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "GetOAuthUserID")
	defer span.End()

	key := oauthPrefix + state
	cmd := r.valkey.B().Get().Key(key).Build()

	userID, err := r.valkey.Do(ctx, cmd).AsInt64()
	if err != nil {
		return 0, fmt.Errorf("failed to get oauth state: %w", err)
	}

	return int(userID), nil
}

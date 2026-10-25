package panorama

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// ErrUnknownProvider is returned when provided panorama provider is unknown.
var ErrUnknownProvider = errors.New("unknown provider")

// NewStreetviewInArea selects a covered location not already used in the game.
func (uc Usecase) NewStreetviewInArea(
	ctx context.Context,
	provider game.PanoramaProvider,
	area playarea.Selection,
	gameID uuid.UUID,
	multiplayer bool,
) (game.PanoramaMetadata, error) {
	location, err := uc.repo.RandomPlayAreaLocation(ctx, provider, area, gameID, multiplayer)
	if err != nil {
		return game.PanoramaMetadata{}, fmt.Errorf("select unused play area location: %w", err)
	}

	return location, nil
}

// NewStreetview creates a new streetview for provided panorama provider.
func (uc Usecase) NewStreetview(
	ctx context.Context,
	provider game.PanoramaProvider,
) (game.PanoramaMetadata, error) {
	switch provider {
	case game.GoogleProvider:
		return uc.NewGoogleStreetview(ctx)
	case game.YandexProvider:
		return uc.NewYandexStreetview(ctx)
	case game.YandexAirProvider:
		return uc.NewYandexAirview(ctx)
	case game.SeznamProvider:
		return uc.NewSeznamStreetview(ctx)
	default:
		return game.PanoramaMetadata{}, ErrUnknownProvider
	}
}

// GetStreetview returns a streetview by id for provided panorama provider.
func (uc Usecase) GetStreetview(
	ctx context.Context,
	provider game.PanoramaProvider,
	id uuid.UUID,
) (game.PanoramaMetadata, error) {
	switch provider {
	case game.GoogleProvider:
		return uc.GetGoogleStreetview(ctx, id)
	case game.YandexProvider:
		return uc.GetYandexStreetview(ctx, id)
	case game.YandexAirProvider:
		return uc.GetYandexAirview(ctx, id)
	case game.SeznamProvider:
		return uc.GetSeznamStreetview(ctx, id)
	default:
		return game.PanoramaMetadata{}, ErrUnknownProvider
	}
}

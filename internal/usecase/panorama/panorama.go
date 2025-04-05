package panorama

import (
	"context"
	"errors"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// ErrUnknownProvider is returned when provided panorama provider is unknown.
var ErrUnknownProvider = errors.New("unknown provider")

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

// GetStreetview returns a streetview by ID for provided panorama provider.
func (uc Usecase) GetStreetview(
	ctx context.Context,
	provider game.PanoramaProvider,
	id int,
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

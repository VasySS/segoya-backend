package panorama

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// NewYandexAirview gets a random airview from the database.
func (uc Usecase) NewYandexAirview(ctx context.Context) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "NewYandexAirview")
	defer span.End()

	panorama, err := uc.repo.RandomYandexAirview(ctx)
	if err != nil {
		span.RecordError(err)
		return game.PanoramaMetadata{}, fmt.Errorf("failed to get random yandex airview from db: %w", err)
	}

	return game.PanoramaMetadata{
		ID:           panorama.ID,
		StreetviewID: panorama.StreetviewID,
		LatLng:       game.LatLng{Lat: panorama.Lat, Lng: panorama.Lng},
	}, nil
}

// GetYandexAirview gets an airview by ID from the database.
func (uc Usecase) GetYandexAirview(ctx context.Context, id int) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "GetYandexAirview")
	defer span.End()

	panorama, err := uc.repo.GetYandexAirview(ctx, id)
	if err != nil {
		span.RecordError(err)
		return game.PanoramaMetadata{}, fmt.Errorf("failed to get panorama: %w", err)
	}

	return game.PanoramaMetadata{
		ID:           panorama.ID,
		StreetviewID: panorama.StreetviewID,
		LatLng:       game.LatLng{Lat: panorama.Lat, Lng: panorama.Lng},
	}, nil
}

// NewYandexStreetview gets a random streetview from the database.
func (uc Usecase) NewYandexStreetview(ctx context.Context) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "NewYandexStreetview")
	defer span.End()

	panorama, err := uc.repo.RandomYandexStreetview(ctx)
	if err != nil {
		span.RecordError(err)
		return game.PanoramaMetadata{}, fmt.Errorf("failed to get random yandex streetview from db: %w", err)
	}

	return game.PanoramaMetadata{
		ID:           panorama.ID,
		StreetviewID: "",
		LatLng:       game.LatLng{Lat: panorama.Lat, Lng: panorama.Lng},
	}, nil
}

// GetYandexStreetview gets a streetview by ID from the database.
func (uc Usecase) GetYandexStreetview(ctx context.Context, id int) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "GetYandexStreetview")
	defer span.End()

	panorama, err := uc.repo.GetYandexStreetview(ctx, id)
	if err != nil {
		span.RecordError(err)
		return game.PanoramaMetadata{}, fmt.Errorf("failed to get panorama: %w", err)
	}

	return game.PanoramaMetadata{
		ID:           panorama.ID,
		StreetviewID: "",
		LatLng:       game.LatLng{Lat: panorama.Lat, Lng: panorama.Lng},
	}, nil
}

package panorama

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// NewGoogleStreetview gets a random streetview from the database.
func (uc Usecase) NewGoogleStreetview(ctx context.Context) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "NewGoogleStreetview")
	defer span.End()

	panorama, err := uc.repo.RandomGoogleStreetview(ctx)
	if err != nil {
		span.RecordError(err)
		return game.PanoramaMetadata{}, fmt.Errorf("failed to get random google point from db: %w", err)
	}

	return game.PanoramaMetadata{
		ID:           panorama.ID,
		StreetviewID: "",
		LatLng:       game.LatLng{Lat: panorama.Lat, Lng: panorama.Lng},
	}, nil
}

// GetGoogleStreetview gets a streetview by ID from the database.
func (uc Usecase) GetGoogleStreetview(ctx context.Context, id int) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGoogleStreetview")
	defer span.End()

	panorama, err := uc.repo.GetGoogleStreetview(ctx, id)
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

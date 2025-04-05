package panorama

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// NewSeznamStreetview gets a random streetview from the database.
func (uc Usecase) NewSeznamStreetview(ctx context.Context) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "NewSeznamStreetView")
	defer span.End()

	panorama, err := uc.repo.RandomSeznamStreetview(ctx)
	if err != nil {
		span.RecordError(err)
		return game.PanoramaMetadata{}, fmt.Errorf("failed to get random seznam point from db: %w", err)
	}

	return game.PanoramaMetadata{
		ID:           panorama.ID,
		StreetviewID: "",
		LatLng:       game.LatLng{Lat: panorama.Lat, Lng: panorama.Lng},
	}, nil
}

// GetSeznamStreetview gets a streetview by ID from the database.
func (uc Usecase) GetSeznamStreetview(ctx context.Context, id int) (game.PanoramaMetadata, error) {
	ctx, span := uc.tracer.Start(ctx, "GetSeznamStreetview")
	defer span.End()

	panorama, err := uc.repo.GetSeznamStreetview(ctx, id)
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

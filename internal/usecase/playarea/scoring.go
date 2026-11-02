package playarea

import (
	"context"
	"fmt"
	"time"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	area "github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// ResolveScoring uses an applied snapshot, without reopening a saved area or rechecking its owner.
func (uc *Usecase) ResolveScoring(
	ctx context.Context,
	provider game.PanoramaProvider,
	selection area.Selection,
) (game.ScoringSettings, error) {
	ctx, span := uc.tracer.Start(ctx, "ResolveScoring")
	defer span.End()

	if !provider.IsSupported() {
		return game.ScoringSettings{}, game.ErrInvalidProvider
	}

	selection, err := selection.Normalize()
	if err != nil {
		return game.ScoringSettings{}, fmt.Errorf("normalize scoring area: %w", err)
	}

	if selection.Type == "all" {
		return game.ProviderScoring(provider), nil
	}

	var bounds game.GeographicBounds

	switch selection.Type {
	case "polygon":
		ring := selection.Polygon.Coordinates[0]

		bounds = game.GeographicBounds{MinLng: ring[0][0], MaxLng: ring[0][0], MinLat: ring[0][1], MaxLat: ring[0][1]}

		for _, point := range ring[1:] {
			bounds.MinLng, bounds.MaxLng = min(bounds.MinLng, point[0]), max(bounds.MaxLng, point[0])
			bounds.MinLat, bounds.MaxLat = min(bounds.MinLat, point[1]), max(bounds.MaxLat, point[1])
		}
	case "presets":
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		bounds, err = uc.repo.GetPlayAreaPresetBounds(ctx, provider, selection.PresetIDs)
		if err != nil {
			return game.ScoringSettings{}, fmt.Errorf("get scoring boundary: %w", err)
		}
	default:
		return game.ScoringSettings{}, area.ErrInvalidSelection
	}

	scoring, err := game.AreaScoring(bounds)
	if err != nil {
		return game.ScoringSettings{}, fmt.Errorf("derive area scoring: %w", err)
	}

	return scoring, nil
}

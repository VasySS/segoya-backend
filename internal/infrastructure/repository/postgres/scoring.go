package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// GetPlayAreaPresetBounds measures full source boundaries, including disconnected parts.
func (r *Repository) GetPlayAreaPresetBounds(
	ctx context.Context,
	provider game.PanoramaProvider,
	ids []uuid.UUID,
) (game.GeographicBounds, error) {
	ctx, span := r.tracer.Start(ctx, "GetPlayAreaPresetBounds")
	defer span.End()

	selection, err := (playarea.Selection{Type: "presets", PresetIDs: ids}).Normalize()
	if err != nil || !provider.IsSupported() {
		return game.GeographicBounds{}, playarea.ErrInvalidSelection
	}

	query := `
		WITH boundaries AS (
			SELECT COUNT(*) AS total, ST_Extent(geom)::box3d AS ordinary,
				ST_Extent(ST_ShiftLongitude(geom))::box3d AS shifted
			FROM play_area_preset WHERE id = ANY(@ids::uuid[]) AND @provider = ANY(providers)
		)
		SELECT total, COALESCE(ST_XMin(ordinary),0), COALESCE(ST_YMin(ordinary),0),
			COALESCE(ST_XMax(ordinary),0), COALESCE(ST_YMax(ordinary),0),
			COALESCE(ST_XMin(shifted),0), COALESCE(ST_XMax(shifted),0)
		FROM boundaries
	`

	var (
		bounds                 game.GeographicBounds
		count                  int
		shiftedMin, shiftedMax float64
	)

	err = r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query,
		pgx.NamedArgs{"ids": selection.PresetIDs, "provider": string(provider)}).
		Scan(&count, &bounds.MinLng, &bounds.MinLat, &bounds.MaxLng, &bounds.MaxLat, &shiftedMin, &shiftedMax)
	if err != nil {
		return game.GeographicBounds{}, fmt.Errorf("get preset scoring bounds: %w", err)
	}

	if count != len(selection.PresetIDs) {
		return game.GeographicBounds{}, playarea.ErrInvalidSelection
	}

	if shiftedMax-shiftedMin < bounds.MaxLng-bounds.MinLng {
		bounds.MinLng, bounds.MaxLng = shiftedMin, shiftedMax
	}

	return bounds, nil
}

// InitializeSingleplayerScoring freezes settings while the caller holds the game lock.
func (r *Repository) InitializeSingleplayerScoring(
	ctx context.Context,
	id uuid.UUID,
	scoring game.ScoringSettings,
) error {
	return r.initializeScoring(ctx, id, scoring, false)
}

// InitializeMultiplayerScoring freezes settings while the caller holds the game lock.
func (r *Repository) InitializeMultiplayerScoring(
	ctx context.Context,
	id uuid.UUID,
	scoring game.ScoringSettings,
) error {
	return r.initializeScoring(ctx, id, scoring, true)
}

func (r *Repository) initializeScoring(
	ctx context.Context,
	id uuid.UUID,
	scoring game.ScoringSettings,
	multiplayer bool,
) error {
	if err := scoring.Validate(); err != nil {
		return fmt.Errorf("validate game scoring: %w", err)
	}

	table := "singleplayer_game"
	if multiplayer {
		table = "multiplayer_game"
	}

	// The table name comes only from the fixed allowlist above.
	query := `UPDATE ` + table + `
		SET scoring_version = @version, scoring_scale_meters = @scale
		WHERE id = @id AND scoring_version IS NULL AND scoring_scale_meters IS NULL AND NOT finished`

	result, err := r.txManager.GetQueryEngine(ctx).Exec(ctx, query, pgx.NamedArgs{
		"id": id, "version": scoring.Version, "scale": scoring.ScaleMeters,
	})
	if err != nil {
		return fmt.Errorf("initialize game scoring: %w", err)
	}

	if result.RowsAffected() != 1 {
		return game.ErrInvalidState
	}

	return nil
}

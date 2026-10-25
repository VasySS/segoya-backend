package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// playAreaPredicate is shared by counts and round selection. Only fixed SQL fragments
// are interpolated; selection values are always bound parameters.
func playAreaPredicate(area playarea.Selection, args pgx.NamedArgs) (string, error) {
	area, err := area.Normalize()
	if err != nil {
		return "", fmt.Errorf("normalize play area: %w", err)
	}

	switch area.Type {
	case "all":
		return "TRUE", nil
	case "presets":
		args["play_area_preset_ids"] = area.PresetIDs

		return `EXISTS (
			SELECT 1 FROM play_area_preset_location pal
			WHERE pal.location_id = pl.id AND pal.preset_id = ANY(@play_area_preset_ids::uuid[])
		)`, nil
	case "polygon":
		polygon, err := json.Marshal(area.Polygon)
		if err != nil {
			return "", fmt.Errorf("encode play area polygon: %w", err)
		}

		args["play_area_polygon"] = string(polygon)

		return "ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON(@play_area_polygon::jsonb), 4326), pl.geom)", nil
	default:
		// Saved selections must be resolved with the user's access rights before querying.
		return "", playarea.ErrInvalidSelection
	}
}

// CountPlayAreaLocations counts distinct provider locations covered by the selection.
func (r *Repository) CountPlayAreaLocations(
	ctx context.Context,
	provider game.PanoramaProvider,
	area playarea.Selection,
) (int, error) {
	ctx, span := r.tracer.Start(ctx, "CountPlayAreaLocations")
	defer span.End()

	if !provider.IsSupported() {
		return 0, playarea.ErrInvalidSelection
	}

	if area.Type == "presets" {
		if err := r.ValidatePlayAreaPresets(ctx, provider, area.PresetIDs); err != nil {
			return 0, err
		}
	}

	if area.Type == "polygon" && area.Polygon != nil {
		if err := r.validatePlayAreaPolygon(ctx, *area.Polygon); err != nil {
			return 0, err
		}
	}

	args := pgx.NamedArgs{"provider": string(provider)}

	predicate, err := playAreaPredicate(area, args)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*) FROM panorama_location pl
		WHERE pl.provider = @provider AND ` + predicate

	var count int

	err = r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query, args).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count play area locations: %w", err)
	}

	return count, nil
}

// ValidatePlayAreaPresets rejects missing presets and provider-incompatible selections.
func (r *Repository) ValidatePlayAreaPresets(
	ctx context.Context,
	provider game.PanoramaProvider,
	ids []uuid.UUID,
) error {
	ctx, span := r.tracer.Start(ctx, "ValidatePlayAreaPresets")
	defer span.End()

	area, err := (playarea.Selection{Type: "presets", PresetIDs: ids}).Normalize()
	if err != nil || !provider.IsSupported() {
		return playarea.ErrInvalidSelection
	}

	query := `
		SELECT COUNT(*) FROM play_area_preset
		WHERE id = ANY(@ids::uuid[]) AND @provider = ANY(providers)
	`

	var count int

	err = r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query,
		pgx.NamedArgs{"ids": area.PresetIDs, "provider": string(provider)}).Scan(&count)
	if err != nil {
		return fmt.Errorf("validate play area presets: %w", err)
	}

	if count != len(area.PresetIDs) {
		return playarea.ErrInvalidSelection
	}

	return nil
}

// RandomPlayAreaLocation selects an unused start location using the same filter as counting.
// The caller holds the game lock and inserts the round in its current transaction.
func (r *Repository) RandomPlayAreaLocation(
	ctx context.Context,
	provider game.PanoramaProvider,
	area playarea.Selection,
	gameID uuid.UUID,
	multiplayer bool,
) (game.PanoramaMetadata, error) {
	ctx, span := r.tracer.Start(ctx, "RandomPlayAreaLocation")
	defer span.End()

	if !provider.IsSupported() {
		return game.PanoramaMetadata{}, playarea.ErrInvalidSelection
	}

	if area.Type == "polygon" && area.Polygon != nil {
		if err := r.validatePlayAreaPolygon(ctx, *area.Polygon); err != nil {
			return game.PanoramaMetadata{}, err
		}
	}

	args := pgx.NamedArgs{"provider": string(provider), "game_id": gameID}

	predicate, err := playAreaPredicate(area, args)
	if err != nil {
		return game.PanoramaMetadata{}, err
	}

	roundTable := "singleplayer_round"
	if multiplayer {
		roundTable = "multiplayer_round"
	}

	// roundTable comes exclusively from the fixed allowlist above.
	query := `
		SELECT pl.id, COALESCE(pl.streetview_id, ''), pl.lat, pl.lng
		FROM panorama_location pl
		WHERE pl.provider = @provider AND ` + predicate + `
		AND NOT EXISTS (
			SELECT 1 FROM ` + roundTable + ` pr WHERE pr.game_id = @game_id AND pr.location_id = pl.id
		)
		ORDER BY RANDOM() LIMIT 1
	`

	var location game.PanoramaMetadata

	err = r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query, args).
		Scan(&location.ID, &location.StreetviewID, &location.Lat, &location.Lng)
	if errors.Is(err, pgx.ErrNoRows) {
		return game.PanoramaMetadata{}, playarea.ErrInsufficientLocations
	}

	if err != nil {
		return game.PanoramaMetadata{}, fmt.Errorf("select play area location: %w", err)
	}

	return location, nil
}

const presetColumns = `p.id, p.name, p.kind, p.country_code, p.version,
	p.source, p.license, p.attribution, p.providers,
	ARRAY[ST_XMin(p.geom::box3d), ST_YMin(p.geom::box3d),
	      ST_XMax(p.geom::box3d), ST_YMax(p.geom::box3d)]`

// ListPlayAreaPresets lists provider-compatible catalog entries and current location counts.
func (r *Repository) ListPlayAreaPresets(
	ctx context.Context,
	provider game.PanoramaProvider,
) ([]playarea.Preset, error) {
	ctx, span := r.tracer.Start(ctx, "ListPlayAreaPresets")
	defer span.End()

	if !provider.IsSupported() {
		return nil, playarea.ErrInvalidSelection
	}

	query := `
		SELECT ` + presetColumns + `, (
			SELECT COUNT(*) FROM play_area_preset_location pal
			JOIN panorama_location pl ON pl.id = pal.location_id
			WHERE pal.preset_id = p.id AND pl.provider = @provider::panorama_provider
		)
		FROM play_area_preset p WHERE @provider::text = ANY(p.providers)
		ORDER BY p.name, p.id
	`

	rows, err := r.txManager.GetQueryEngine(ctx).Query(ctx, query, pgx.NamedArgs{"provider": string(provider)})
	if err != nil {
		return nil, fmt.Errorf("list play area presets: %w", err)
	}
	defer rows.Close()

	presets := []playarea.Preset{}

	for rows.Next() {
		var preset playarea.Preset

		if err := scanPreset(rows, &preset, &preset.LocationCount); err != nil {
			return nil, fmt.Errorf("scan play area preset: %w", err)
		}

		presets = append(presets, preset)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate play area presets: %w", err)
	}

	return presets, nil
}

// GetPlayAreaPreset returns a preset's display outline and its source attribution.
func (r *Repository) GetPlayAreaPreset(ctx context.Context, id uuid.UUID) (playarea.Preset, error) {
	ctx, span := r.tracer.Start(ctx, "GetPlayAreaPreset")
	defer span.End()

	query := `
		SELECT ` + presetColumns + `, ST_AsGeoJSON(p.display_geom)::jsonb
		FROM play_area_preset p WHERE p.id = @id
	`

	var preset playarea.Preset

	row := r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query, pgx.NamedArgs{"id": id})

	err := scanPreset(row, &preset, &preset.Polygon)
	if errors.Is(err, pgx.ErrNoRows) {
		return playarea.Preset{}, playarea.ErrNotFound
	}

	if err != nil {
		return playarea.Preset{}, fmt.Errorf("get play area preset: %w", err)
	}

	return preset, nil
}

type playAreaScanner interface {
	Scan(dest ...any) error
}

func scanPreset(row playAreaScanner, preset *playarea.Preset, last any) error {
	if err := row.Scan(
		&preset.ID, &preset.Name, &preset.Kind, &preset.CountryCode, &preset.Version,
		&preset.Source, &preset.License, &preset.Attribution, &preset.Providers, &preset.Bounds, last,
	); err != nil {
		return fmt.Errorf("scan preset row: %w", err)
	}

	return nil
}

const savedPlayAreaColumns = `id, owner_id, name, polygon, visibility, created_at, updated_at`

// CreatePlayArea saves a provider-independent polygon in the user's library.
func (r *Repository) CreatePlayArea(
	ctx context.Context,
	owner uuid.UUID,
	name string,
	polygon playarea.Polygon,
	visibility string,
) (playarea.Saved, error) {
	ctx, span := r.tracer.Start(ctx, "CreatePlayArea")
	defer span.End()

	name = strings.TrimSpace(name)
	if err := playarea.ValidateName(name); err != nil {
		return playarea.Saved{}, fmt.Errorf("validate saved play area name: %w", err)
	}

	if visibility == "" {
		visibility = "private"
	}

	if err := playarea.ValidateVisibility(visibility); err != nil {
		return playarea.Saved{}, fmt.Errorf("validate saved play area visibility: %w", err)
	}

	if err := r.validatePlayAreaPolygon(ctx, polygon); err != nil {
		return playarea.Saved{}, err
	}

	query := `
		INSERT INTO saved_play_area (owner_id, name, polygon, visibility)
		VALUES (@owner, @name, @polygon, @visibility)
		RETURNING ` + savedPlayAreaColumns

	var saved playarea.Saved

	row := r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query, pgx.NamedArgs{
		"owner": owner, "name": name, "polygon": polygon, "visibility": visibility,
	})
	if err := scanSavedPlayArea(row, &saved); err != nil {
		return playarea.Saved{}, fmt.Errorf("create saved play area: %w", err)
	}

	return saved, nil
}

// GetPlayArea reads an owner's area or an unlisted area shared by ID.
func (r *Repository) GetPlayArea(ctx context.Context, id, user uuid.UUID) (playarea.Saved, error) {
	ctx, span := r.tracer.Start(ctx, "GetPlayArea")
	defer span.End()

	query := `
		SELECT ` + savedPlayAreaColumns + ` FROM saved_play_area
		WHERE id = @id AND (owner_id = @user OR visibility = 'unlisted')
	`

	var saved playarea.Saved

	row := r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query, pgx.NamedArgs{"id": id, "user": user})

	err := scanSavedPlayArea(row, &saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return playarea.Saved{}, playarea.ErrNotFound
	}

	if err != nil {
		return playarea.Saved{}, fmt.Errorf("get saved play area: %w", err)
	}

	return saved, nil
}

// ListPlayAreas lists only the owner's library, including its total size.
func (r *Repository) ListPlayAreas(
	ctx context.Context,
	owner uuid.UUID,
	page, pageSize int,
) ([]playarea.Saved, int, error) {
	ctx, span := r.tracer.Start(ctx, "ListPlayAreas")
	defer span.End()

	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, 0, playarea.ErrInvalidSelection
	}

	tx := r.txManager.GetQueryEngine(ctx)

	countQuery := `SELECT COUNT(*) FROM saved_play_area WHERE owner_id = @owner`

	var total int

	if err := tx.QueryRow(ctx, countQuery,
		pgx.NamedArgs{"owner": owner}).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count saved play areas: %w", err)
	}

	query := `
		SELECT ` + savedPlayAreaColumns + ` FROM saved_play_area WHERE owner_id = @owner
		ORDER BY created_at DESC, id DESC LIMIT @limit OFFSET @offset
	`

	rows, err := tx.Query(ctx, query,
		pgx.NamedArgs{"owner": owner, "limit": pageSize, "offset": int64(page-1) * int64(pageSize)})
	if err != nil {
		return nil, 0, fmt.Errorf("list saved play areas: %w", err)
	}
	defer rows.Close()

	areas := []playarea.Saved{}

	for rows.Next() {
		var saved playarea.Saved

		if err := scanSavedPlayArea(rows, &saved); err != nil {
			return nil, 0, fmt.Errorf("scan saved play area: %w", err)
		}

		areas = append(areas, saved)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate saved play areas: %w", err)
	}

	return areas, total, nil
}

// UpdatePlayArea atomically changes supplied fields only when the user owns the area.
func (r *Repository) UpdatePlayArea(
	ctx context.Context,
	id, owner uuid.UUID,
	name *string,
	polygon *playarea.Polygon,
	visibility *string,
) (playarea.Saved, error) {
	ctx, span := r.tracer.Start(ctx, "UpdatePlayArea")
	defer span.End()

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if err := playarea.ValidateName(trimmed); err != nil {
			return playarea.Saved{}, fmt.Errorf("validate saved play area name: %w", err)
		}

		name = &trimmed
	}

	if polygon != nil {
		if err := r.validatePlayAreaPolygon(ctx, *polygon); err != nil {
			return playarea.Saved{}, err
		}
	}

	if visibility != nil {
		if err := playarea.ValidateVisibility(*visibility); err != nil {
			return playarea.Saved{}, fmt.Errorf("validate saved play area visibility: %w", err)
		}
	}

	query := `
		UPDATE saved_play_area SET name = COALESCE(@name, name),
			polygon = COALESCE(@polygon::jsonb, polygon),
			visibility = COALESCE(@visibility, visibility), updated_at = clock_timestamp()
		WHERE id = @id AND owner_id = @owner
		RETURNING ` + savedPlayAreaColumns

	var saved playarea.Saved

	row := r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query, pgx.NamedArgs{
		"id": id, "owner": owner, "name": name, "polygon": polygon, "visibility": visibility,
	})

	err := scanSavedPlayArea(row, &saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return playarea.Saved{}, playarea.ErrNotFound
	}

	if err != nil {
		return playarea.Saved{}, fmt.Errorf("update saved play area: %w", err)
	}

	return saved, nil
}

// DeletePlayArea removes an area only when the user owns it.
func (r *Repository) DeletePlayArea(ctx context.Context, id, owner uuid.UUID) error {
	ctx, span := r.tracer.Start(ctx, "DeletePlayArea")
	defer span.End()

	query := `
		DELETE FROM saved_play_area WHERE id = @id AND owner_id = @owner
	`

	result, err := r.txManager.GetQueryEngine(ctx).Exec(ctx, query, pgx.NamedArgs{"id": id, "owner": owner})
	if err != nil {
		return fmt.Errorf("delete saved play area: %w", err)
	}

	if result.RowsAffected() == 0 {
		return playarea.ErrNotFound
	}

	return nil
}

func scanSavedPlayArea(row playAreaScanner, saved *playarea.Saved) error {
	if err := row.Scan(&saved.ID, &saved.OwnerID, &saved.Name, &saved.Polygon,
		&saved.Visibility, &saved.CreatedAt, &saved.UpdatedAt); err != nil {
		return fmt.Errorf("scan saved play area row: %w", err)
	}

	return nil
}

func (r *Repository) validatePlayAreaPolygon(ctx context.Context, polygon playarea.Polygon) error {
	if err := polygon.Validate(); err != nil {
		return fmt.Errorf("validate play area polygon: %w", err)
	}

	query := `SELECT ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON(@polygon::jsonb), 4326))`

	var valid bool

	err := r.txManager.GetQueryEngine(ctx).QueryRow(ctx, query,
		pgx.NamedArgs{"polygon": polygon}).Scan(&valid)
	if err != nil {
		return fmt.Errorf("validate play area topology: %w", err)
	}

	if !valid {
		return playarea.ErrInvalidPolygon
	}

	return nil
}

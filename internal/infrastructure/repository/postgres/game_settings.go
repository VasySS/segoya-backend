package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// gameSettingsRow maps native PostgreSQL fields to the existing domain types.
type gameSettingsRow struct {
	PresetIDs          []uuid.UUID `db:"play_area_preset_ids"`
	Polygon            *string     `db:"play_area_polygon"`
	ScoringVersion     *int        `db:"scoring_version"`
	ScoringScaleMeters *float64    `db:"scoring_scale_meters"`
}

type singleplayerGameRow struct {
	singleplayer.Game
	gameSettingsRow
}

type multiplayerGameRow struct {
	multiplayer.Game
	gameSettingsRow
}

func encodeGameSettings(area playarea.Selection, scoring *game.ScoringSettings) (gameSettingsRow, error) {
	area, err := area.Normalize()
	if err != nil {
		return gameSettingsRow{}, fmt.Errorf("normalize stored play area: %w", err)
	}

	if area.Type == "saved" {
		return gameSettingsRow{}, playarea.ErrInvalidSelection
	}

	row := gameSettingsRow{PresetIDs: append([]uuid.UUID{}, area.PresetIDs...)}

	if area.Polygon != nil {
		row.Polygon, err = encodePolygon(*area.Polygon)
		if err != nil {
			return gameSettingsRow{}, err
		}
	}

	if scoring != nil {
		if err := scoring.Validate(); err != nil {
			return gameSettingsRow{}, fmt.Errorf("validate stored scoring: %w", err)
		}

		row.ScoringVersion, row.ScoringScaleMeters = &scoring.Version, &scoring.ScaleMeters
	}

	return row, nil
}

func (r gameSettingsRow) decode() (playarea.Selection, *game.ScoringSettings, error) {
	area := playarea.Selection{Type: "all"}
	if len(r.PresetIDs) > 0 {
		area.Type, area.PresetIDs = "presets", r.PresetIDs
	}

	if r.Polygon != nil {
		polygon, err := decodePolygon(*r.Polygon)
		if err != nil {
			return playarea.Selection{}, nil, err
		}

		area.Type, area.Polygon = "polygon", &polygon
	}

	area, err := area.Normalize()
	if err != nil {
		return playarea.Selection{}, nil, fmt.Errorf("decode stored play area: %w", err)
	}

	scoring, err := r.scoring()
	if err != nil {
		return playarea.Selection{}, nil, err
	}

	return area, scoring, nil
}

//nolint:nilnil // NULL scoring fields represent legacy games awaiting initialization.
func (r gameSettingsRow) scoring() (*game.ScoringSettings, error) {
	if r.ScoringVersion == nil && r.ScoringScaleMeters == nil {
		return nil, nil
	}

	if r.ScoringVersion == nil || r.ScoringScaleMeters == nil {
		return nil, game.ErrInvalidScoring
	}

	settings := game.ScoringSettings{Version: *r.ScoringVersion, ScaleMeters: *r.ScoringScaleMeters}
	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("decode stored scoring: %w", err)
	}

	return &settings, nil
}

func (r singleplayerGameRow) toGame() (singleplayer.Game, error) {
	var err error

	r.PlayArea, r.Scoring, err = r.decode()
	if err != nil {
		return singleplayer.Game{}, err
	}

	return r.Game, nil
}

func (r multiplayerGameRow) toGame() (multiplayer.Game, error) {
	var err error

	r.PlayArea, r.Scoring, err = r.decode()
	if err != nil {
		return multiplayer.Game{}, err
	}

	return r.Game, nil
}

func encodePolygon(polygon playarea.Polygon) (*string, error) {
	encoded, err := json.Marshal(polygon)
	if err != nil {
		return nil, fmt.Errorf("encode polygon: %w", err)
	}

	value := string(encoded)

	return &value, nil
}

// Geometry queries use 25 decimal places: 17 significant digits plus up to eight
// leading fractional zeroes before PostGIS switches to scientific notation.
func decodePolygon(encoded string) (playarea.Polygon, error) {
	var polygon playarea.Polygon

	if err := json.Unmarshal([]byte(encoded), &polygon); err != nil {
		return playarea.Polygon{}, fmt.Errorf("decode polygon: %w", err)
	}

	if err := polygon.Validate(); err != nil {
		return playarea.Polygon{}, fmt.Errorf("validate stored polygon: %w", err)
	}

	return polygon, nil
}

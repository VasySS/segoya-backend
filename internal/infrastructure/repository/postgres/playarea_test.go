//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/migrations/tables"
)

func TestPlayAreaTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PlayAreaTestSuite))
}

type PlayAreaTestSuite struct{ postgresTestSuite }

func playAreaSquare() playarea.Polygon {
	return playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{
		{20, 10}, {22, 10}, {22, 12}, {20, 12}, {20, 10},
	}}}
}

func (s *PlayAreaTestSuite) TestCountPlayAreaLocations() {
	s.addSpatialLocations()

	polygon := playAreaSquare()
	area := playarea.Selection{Type: "polygon", Polygon: &polygon}

	cases := []struct {
		name     string
		provider game.PanoramaProvider
		area     playarea.Selection
		count    int
	}{
		{name: "inside edge and corner are included", provider: game.GoogleProvider, area: area, count: 3},
		{name: "provider isolation", provider: game.YandexProvider, area: area, count: 1},
		{name: "empty provider coverage", provider: game.SeznamProvider, area: area, count: 0},
		{name: "legacy zero value covers every provider location", provider: game.GoogleProvider, count: 5},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			count, err := s.postgresRepo.CountPlayAreaLocations(s.ctx, tc.provider, tc.area)
			s.Require().NoError(err)
			s.Equal(tc.count, count)
		})
	}

	// Swapping longitude and latitude must select a different part of the world.
	for _, point := range polygon.Coordinates[0] {
		point[0], point[1] = point[1], point[0]
	}

	count, err := s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider, area)
	s.Require().NoError(err)
	s.Zero(count)

	invalid := playAreaSquare()
	invalid.Coordinates[0][2] = []float64{20, 9}
	_, err = s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider,
		playarea.Selection{Type: "polygon", Polygon: &invalid})
	s.Require().ErrorIs(err, playarea.ErrInvalidPolygon)
	_, err = s.postgresRepo.CountPlayAreaLocations(s.ctx, "unknown", area)
	s.Require().ErrorIs(err, playarea.ErrInvalidSelection)
	_, err = s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider,
		playarea.Selection{Type: "saved", SavedID: uuid.New()})
	s.Require().ErrorIs(err, playarea.ErrInvalidSelection)
}

func (s *PlayAreaTestSuite) TestPresetUnionAndProviderValidation() {
	s.addSpatialLocations()

	first, second := uuid.New(), uuid.New()
	_, err := s.pool.Exec(s.ctx, `
		INSERT INTO play_area_preset
		(id, name, kind, country_code, version, source, license, attribution, providers, geom, display_geom)
		SELECT $1, 'First', 'country', 'ZZZ', 'test-v1', 'test source', 'test license',
		'test attribution', ARRAY['google'], geom, geom
		FROM (SELECT ST_Multi(ST_GeomFromText('POLYGON((20 10,22 10,22 12,20 12,20 10))',4326)) geom) g
	`, first)
	s.Require().NoError(err)
	_, err = s.pool.Exec(s.ctx, `
		INSERT INTO play_area_preset
		SELECT $1, 'Second', kind, country_code, version, source, license, attribution,
		ARRAY['google','yandex'], geom, display_geom FROM play_area_preset WHERE id=$2
	`, second, first)
	s.Require().NoError(err)

	backfill := `INSERT INTO play_area_preset_location (preset_id, location_id)
		SELECT p.id, pl.id FROM play_area_preset p JOIN panorama_location pl
		ON pl.provider::text = ANY(p.providers) AND ST_Covers(p.geom, pl.geom)
		ON CONFLICT DO NOTHING`
	_, err = s.pool.Exec(s.ctx, backfill)
	s.Require().NoError(err)
	_, err = s.pool.Exec(s.ctx, backfill)
	s.Require().NoError(err)

	area := playarea.Selection{Type: "presets", PresetIDs: []uuid.UUID{first, second, first}}
	count, err := s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider, area)
	s.Require().NoError(err)
	s.Equal(3, count, "overlapping and repeated presets must not double-count locations")

	err = s.postgresRepo.ValidatePlayAreaPresets(s.ctx, game.YandexProvider, []uuid.UUID{first})
	s.Require().ErrorIs(err, playarea.ErrInvalidSelection)
	err = s.postgresRepo.ValidatePlayAreaPresets(s.ctx, game.GoogleProvider, []uuid.UUID{uuid.New()})
	s.Require().ErrorIs(err, playarea.ErrInvalidSelection)

	presets, err := s.postgresRepo.ListPlayAreaPresets(s.ctx, game.GoogleProvider)
	s.Require().NoError(err)
	s.Require().Len(presets, 2)
	s.Equal(3, presets[0].LocationCount)
	s.Equal([]float64{20, 10, 22, 12}, presets[0].Bounds)
	s.Empty(presets[0].Polygon, "listing does not transfer display geometry")
	yandexPresets, err := s.postgresRepo.ListPlayAreaPresets(s.ctx, game.YandexProvider)
	s.Require().NoError(err)
	s.Require().Len(yandexPresets, 1)
	s.Equal(1, yandexPresets[0].LocationCount)

	preset, err := s.postgresRepo.GetPlayAreaPreset(s.ctx, first)
	s.Require().NoError(err)
	s.Equal("test attribution", preset.Attribution)
	s.Equal("test-v1", preset.Version)

	var outline struct {
		Type string `json:"type"`
	}

	s.Require().NoError(json.Unmarshal(preset.Polygon, &outline))
	s.Equal("MultiPolygon", outline.Type)
	_, err = s.postgresRepo.GetPlayAreaPreset(s.ctx, uuid.New())
	s.Require().ErrorIs(err, playarea.ErrNotFound)
}

func (s *PlayAreaTestSuite) TestRandomPlayAreaLocationExcludesUsedRounds() {
	ids := s.addSpatialLocations()
	owner := s.addPlayAreaOwner()
	polygon := playAreaSquare()
	area := playarea.Selection{Type: "polygon", Polygon: &polygon}

	for _, multiplayer := range []bool{false, true} {
		name, roundTable := "singleplayer", "singleplayer_round"
		if multiplayer {
			name, roundTable = "multiplayer", "multiplayer_round"
		}

		s.Run(name, func() {
			gameID := uuid.New()

			query := `INSERT INTO singleplayer_game
				(id,user_id,rounds,provider,movement_allowed,timer_seconds,created_at)
				VALUES($1,$2,3,'google',true,30,CURRENT_TIMESTAMP)`
			if multiplayer {
				query = `INSERT INTO multiplayer_game
					(id,creator_id,rounds,provider,movement_allowed,timer_seconds,players,created_at)
					VALUES($1,$2,3,'google',true,30,2,CURRENT_TIMESTAMP)`
			}

			_, err := s.pool.Exec(s.ctx, query, gameID, owner)
			s.Require().NoError(err)

			selected := []uuid.UUID{}

			for round := 1; round <= 3; round++ {
				location, err := s.postgresRepo.RandomPlayAreaLocation(
					s.ctx, game.GoogleProvider, area, gameID, multiplayer,
				)
				s.Require().NoError(err)
				s.Contains(ids, location.ID)
				s.NotContains(selected, location.ID)
				selected = append(selected, location.ID)
				_, err = s.pool.Exec(s.ctx, `INSERT INTO `+roundTable+`
					(game_id,location_id,round_num,created_at,started_at)
					VALUES($1,$2,$3,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, gameID, location.ID, round)
				s.Require().NoError(err)
			}

			_, err = s.postgresRepo.RandomPlayAreaLocation(s.ctx, game.GoogleProvider, area, gameID, multiplayer)
			s.Require().ErrorIs(err, playarea.ErrInsufficientLocations)
		})
	}
}

func (s *PlayAreaTestSuite) TestSavedPlayAreaAccessAndPagination() {
	owner, reader := s.addPlayAreaOwner(), s.addPlayAreaOwner()
	polygon := playAreaSquare()
	saved, err := s.postgresRepo.CreatePlayArea(s.ctx, owner, "  My area  ", polygon, "")
	s.Require().NoError(err)
	s.Equal("My area", saved.Name)
	s.Equal("private", saved.Visibility)
	s.Equal(polygon, saved.Polygon)
	s.False(saved.CreatedAt.IsZero())
	s.Equal(saved.CreatedAt, saved.UpdatedAt)
	_, err = s.postgresRepo.GetPlayArea(s.ctx, saved.ID, reader)
	s.Require().ErrorIs(err, playarea.ErrNotFound)
	_, err = s.postgresRepo.UpdatePlayArea(s.ctx, saved.ID, reader, nil, nil, nil)
	s.Require().ErrorIs(err, playarea.ErrNotFound)
	s.Require().ErrorIs(s.postgresRepo.DeletePlayArea(s.ctx, saved.ID, reader), playarea.ErrNotFound)

	visibility := "unlisted"
	shared, err := s.postgresRepo.UpdatePlayArea(s.ctx, saved.ID, owner, nil, nil, &visibility)
	s.Require().NoError(err)
	s.Equal(polygon, shared.Polygon, "partial updates preserve polygon")
	s.GreaterOrEqual(shared.UpdatedAt.UnixMicro(), saved.UpdatedAt.UnixMicro())
	got, err := s.postgresRepo.GetPlayArea(s.ctx, saved.ID, reader)
	s.Require().NoError(err)
	s.Equal(shared, got)

	_, err = s.postgresRepo.CreatePlayArea(s.ctx, owner, "Second area", polygon, "private")
	s.Require().NoError(err)
	listed, total, err := s.postgresRepo.ListPlayAreas(s.ctx, owner, 1, 1)
	s.Require().NoError(err)
	s.Len(listed, 1)
	s.Equal(2, total)
	listed, total, err = s.postgresRepo.ListPlayAreas(s.ctx, owner, 100, 1)
	s.Require().NoError(err)
	s.Empty(listed)
	s.Equal(2, total)
	listed, total, err = s.postgresRepo.ListPlayAreas(s.ctx, reader, 1, 20)
	s.Require().NoError(err)
	s.Empty(listed)
	s.Zero(total, "shared areas do not appear in the reader's library")

	name := "Updated"
	polygon.Coordinates[0][1][0] = 23
	updated, err := s.postgresRepo.UpdatePlayArea(s.ctx, saved.ID, owner, &name, &polygon, nil)
	s.Require().NoError(err)
	s.Equal(name, updated.Name)
	s.Equal(polygon, updated.Polygon)
	s.Equal("unlisted", updated.Visibility)
	s.Require().NoError(s.postgresRepo.DeletePlayArea(s.ctx, saved.ID, owner))
	_, err = s.postgresRepo.GetPlayArea(s.ctx, saved.ID, owner)
	s.Require().ErrorIs(err, playarea.ErrNotFound)
}

func (s *PlayAreaTestSuite) TestInvalidSavedPlayAreas() {
	owner := s.addPlayAreaOwner()
	polygon := playAreaSquare()
	_, err := s.postgresRepo.CreatePlayArea(s.ctx, owner, " ", polygon, "private")
	s.Require().ErrorIs(err, playarea.ErrInvalidName)
	_, err = s.postgresRepo.CreatePlayArea(s.ctx, owner, "Area", polygon, "public")
	s.Require().ErrorIs(err, playarea.ErrInvalidVisibility)

	polygon.Coordinates[0][2] = []float64{20, 9}
	_, err = s.postgresRepo.CreatePlayArea(s.ctx, owner, "Area", polygon, "private")
	s.Require().ErrorIs(err, playarea.ErrInvalidPolygon)
}

func (s *PlayAreaTestSuite) TestSpatialMigrationCanBeReapplied() {
	ctx := s.ctx
	s.Require().NoError(tables.RunGooseMigrations(ctx, s.pool, "down-to", "11"))
	s.Require().NoError(tables.RunGooseMigrations(ctx, s.pool, "up"))

	var lng, lat float64

	err := s.pool.QueryRow(ctx, `SELECT ST_X(geom),ST_Y(geom) FROM panorama_location WHERE provider='yandex'`).
		Scan(&lng, &lat)
	s.Require().NoError(err)
	s.InDelta(37.625, lng, 0)
	s.InDelta(55.75, lat, 0)
}

// Ensure context cancellation survives the spatial repository boundary.
func (s *PlayAreaTestSuite) TestCountPlayAreaLocationsCanceled() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	_, err := s.postgresRepo.CountPlayAreaLocations(ctx, game.GoogleProvider, playarea.Selection{})
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *PlayAreaTestSuite) addSpatialLocations() []uuid.UUID {
	s.T().Helper()

	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	_, err := s.pool.Exec(s.ctx, `
		INSERT INTO panorama_location (id, provider, lat, lng) VALUES
		($1, 'google', 11, 21), ($2, 'google', 11, 20), ($3, 'google', 12, 22),
		(uuidv7(), 'google', 11, 23), (uuidv7(), 'yandex', 11, 21)
	`, ids[0], ids[1], ids[2])
	s.Require().NoError(err)

	return ids
}

func (s *PlayAreaTestSuite) addPlayAreaOwner() uuid.UUID {
	s.T().Helper()
	req := s.newUserRequest()
	s.Require().NoError(s.postgresRepo.NewUser(s.ctx, req))
	profile, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)

	return profile.ID
}

//go:build integration

package postgres_test

import (
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func precisePolygon() playarea.Polygon {
	longitude, latitude := 20.1234567896, 10.987654321098765

	return playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{
		{longitude, latitude}, {longitude + 1, latitude}, {longitude, latitude + 1}, {longitude, latitude},
	}}}
}

func nearZeroPolygon(longitude float64) playarea.Polygon {
	return playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{
		{longitude, 1}, {1, 1}, {1, 2}, {longitude, 1},
	}}}
}

func (s *PlayAreaTestSuite) TestTypedGameSettingsRoundTrips() {
	polygon := precisePolygon()
	nearZero := nearZeroPolygon(0.0000001234567899876543)
	providerScoring := game.ProviderScoring(game.GoogleProvider)
	areaScoring := game.ScoringSettings{Version: game.ScoringVersionArea, ScaleMeters: 12345.6789012345}
	id := uuid.New()

	cases := []struct {
		name    string
		area    playarea.Selection
		scoring *game.ScoringSettings
	}{
		{name: "legacy whole provider"},
		{name: "whole provider", scoring: &providerScoring},
		{name: "presets", area: playarea.Selection{
			Type: "presets", PresetIDs: []uuid.UUID{id, uuid.New(), id},
		}, scoring: &areaScoring},
		{name: "polygon", area: playarea.Selection{Type: "polygon", Polygon: &polygon}, scoring: &areaScoring},
		{name: "near zero polygon", area: playarea.Selection{Type: "polygon", Polygon: &nearZero}, scoring: &areaScoring},
		{name: "legacy polygon", area: playarea.Selection{Type: "polygon", Polygon: &polygon}},
	}

	for _, multiplayer := range []bool{false, true} {
		for _, tc := range cases {
			s.Run(fmt.Sprintf("multiplayer=%t/%s", multiplayer, tc.name), func() {
				owner := s.addPlayAreaOwner()
				id := s.newSettingsGame(owner, multiplayer, tc.area, tc.scoring)
				expected, err := tc.area.Normalize()
				s.Require().NoError(err)

				if multiplayer {
					stored, err := s.postgresRepo.GetMultiplayerGame(s.ctx, id)
					s.Require().NoError(err)
					s.Equal(expected, stored.PlayArea)
					s.Equal(tc.scoring, stored.Scoring)

					return
				}

				stored, err := s.postgresRepo.GetSingleplayerGame(s.ctx, id)
				s.Require().NoError(err)
				s.Equal(expected, stored.PlayArea)
				s.Equal(tc.scoring, stored.Scoring)
				listed, total, err := s.postgresRepo.GetSingleplayerGames(s.ctx,
					dto.GetSingleplayerGamesRequest{UserID: owner, Page: 1, PageSize: 10})
				s.Require().NoError(err)
				s.Require().Equal(1, total)
				s.Require().Len(listed, 1)
				s.Equal(stored, listed[0])
			})
		}
	}
}

func (s *PlayAreaTestSuite) TestSavedPolygonPrecisionAndBoundaryMembership() {
	owner := s.addPlayAreaOwner()
	polygon := precisePolygon()
	corner := polygon.Coordinates[0][0]
	_, err := s.pool.Exec(s.ctx, "INSERT INTO panorama_location(provider,lng,lat) VALUES ('google',$1,$2)",
		corner[0], corner[1]+0.25)
	s.Require().NoError(err)
	saved, err := s.postgresRepo.CreatePlayArea(s.ctx, owner, "Precise polygon", polygon, "private")
	s.Require().NoError(err)
	s.Equal(polygon, saved.Polygon)
	stored, err := s.postgresRepo.GetPlayArea(s.ctx, saved.ID, owner)
	s.Require().NoError(err)
	s.Equal(polygon, stored.Polygon)
	listed, _, err := s.postgresRepo.ListPlayAreas(s.ctx, owner, 1, 10)
	s.Require().NoError(err)
	s.Require().Len(listed, 1)
	s.Equal(polygon, listed[0].Polygon)

	name := "Renamed precise polygon"
	updated, err := s.postgresRepo.UpdatePlayArea(s.ctx, saved.ID, owner, &name, nil, nil)
	s.Require().NoError(err)
	s.Equal(polygon, updated.Polygon)
	count, err := s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider,
		playarea.Selection{Type: "polygon", Polygon: &updated.Polygon})
	s.Require().NoError(err)
	s.Equal(1, count, "a boundary location stays covered after geometry and GeoJSON round trips")

	for _, longitude := range []float64{
		0.0000001234567899876543, math.Nextafter(1e-8, 1), 1e-18, math.SmallestNonzeroFloat64,
	} {
		nearZero := nearZeroPolygon(longitude)
		_, err := s.pool.Exec(s.ctx, "INSERT INTO panorama_location(provider,lng,lat) VALUES ('google',$1,1)", longitude)
		s.Require().NoError(err)
		expected, err := s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider,
			playarea.Selection{Type: "polygon", Polygon: &nearZero})
		s.Require().NoError(err)
		s.Positive(expected)
		stored, err := s.postgresRepo.CreatePlayArea(s.ctx, owner, "Near Greenwich", nearZero, "private")
		s.Require().NoError(err)
		s.Equal(nearZero, stored.Polygon, "tiny coordinates must retain float64 precision")
		actual, err := s.postgresRepo.CountPlayAreaLocations(s.ctx, game.GoogleProvider,
			playarea.Selection{Type: "polygon", Polygon: &stored.Polygon})
		s.Require().NoError(err)
		s.Equal(expected, actual, "a boundary vertex stays covered near longitude zero")
	}
}

func (s *PlayAreaTestSuite) TestTypedGameSettingsConstraints() {
	owner := s.addPlayAreaOwner()
	cases := []struct {
		name string
		set  string
	}{
		{name: "mixed selection", set: `play_area_preset_ids=ARRAY[uuidv7()],
			play_area_polygon=ST_GeomFromText('POLYGON((0 0,1 0,0 1,0 0))',4326)`},
		{name: "null preset array", set: "play_area_preset_ids=NULL"},
		{name: "empty polygon", set: "play_area_polygon=ST_GeomFromText('POLYGON EMPTY',4326)"},
		{name: "invalid polygon", set: "play_area_polygon=ST_GeomFromText('POLYGON((0 0,1 1,0 1,1 0,0 0))',4326)"},
		{name: "wrong geometry type", set: "play_area_polygon=ST_GeomFromText('POINT(0 0)',4326)"},
		{name: "wrong SRID", set: "play_area_polygon=ST_GeomFromText('POLYGON((0 0,1 0,0 1,0 0))',3857)"},
		{name: "version without scale", set: "scoring_version=1"},
		{name: "scale without version", set: "scoring_scale_meters=1"},
		{name: "unsupported version", set: "scoring_version=3,scoring_scale_meters=1"},
		{name: "zero scale", set: "scoring_version=1,scoring_scale_meters=0"},
		{name: "negative scale", set: "scoring_version=1,scoring_scale_meters=-1"},
		{name: "infinite scale", set: "scoring_version=1,scoring_scale_meters='Infinity'::float8"},
		{name: "negative infinite scale", set: "scoring_version=1,scoring_scale_meters='-Infinity'::float8"},
		{name: "NaN scale", set: "scoring_version=1,scoring_scale_meters='NaN'::float8"},
	}

	for _, multiplayer := range []bool{false, true} {
		id := s.newSettingsGame(owner, multiplayer, playarea.Selection{}, nil)

		table := "singleplayer_game"
		if multiplayer {
			table = "multiplayer_game"
		}

		for _, tc := range cases {
			s.Run(table+"/"+tc.name, func() {
				_, err := s.pool.Exec(s.ctx, "UPDATE "+table+" SET "+tc.set+" WHERE id=$1", id)
				s.Require().Error(err)
			})
		}
	}
}

func (s *PlayAreaTestSuite) TestSavedPolygonGeometryConstraints() {
	owner := s.addPlayAreaOwner()

	for _, tc := range []struct {
		name string
		wkt  string
		srid int
	}{
		{name: "empty", wkt: "POLYGON EMPTY", srid: 4326},
		{name: "invalid", wkt: "POLYGON((0 0,1 1,0 1,1 0,0 0))", srid: 4326},
		{name: "wrong type", wkt: "POINT(0 0)", srid: 4326},
		{name: "wrong SRID", wkt: "POLYGON((0 0,1 0,0 1,0 0))", srid: 3857},
	} {
		s.Run(tc.name, func() {
			query := `INSERT INTO saved_play_area(owner_id,name,polygon)
				VALUES ($1,'Invalid geometry',ST_GeomFromText($2,$3))`
			_, err := s.pool.Exec(s.ctx, query, owner, tc.wkt, tc.srid)
			s.Require().Error(err)
		})
	}
}

func (s *PlayAreaTestSuite) newSettingsGame(
	owner uuid.UUID, multiplayer bool, area playarea.Selection, scoring *game.ScoringSettings,
) uuid.UUID {
	s.T().Helper()

	if multiplayer {
		other := s.addPlayAreaOwner()
		id, err := s.postgresRepo.NewMultiplayerGame(s.ctx, dto.NewMultiplayerGameRequest{
			CreatorID: owner, Provider: "google", Rounds: 3, RequestTime: fixtureTime(), PlayArea: area, Scoring: scoring,
			ConnectedPlayers: []user.PublicProfile{{ID: owner}, {ID: other}},
		})
		s.Require().NoError(err)

		return id
	}

	id, err := s.postgresRepo.NewSingleplayerGame(s.ctx, dto.NewSingleplayerGameRequest{
		UserID: owner, Provider: "google", Rounds: 3, RequestTime: fixtureTime(), PlayArea: area, Scoring: scoring,
	})
	s.Require().NoError(err)

	return id
}

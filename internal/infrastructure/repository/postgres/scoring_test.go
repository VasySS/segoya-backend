//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestScoringTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ScoringTestSuite))
}

type ScoringTestSuite struct{ postgresTestSuite }

func (s *ScoringTestSuite) TestInitializeScoringOnlyOnceForUnfinishedGames() {
	request := s.newUserRequest()
	s.Require().NoError(s.postgresRepo.NewUser(s.ctx, request))
	owner, err := s.postgresRepo.GetUserByUsername(s.ctx, request.Username)
	s.Require().NoError(err)
	request = s.newUserRequest()
	s.Require().NoError(s.postgresRepo.NewUser(s.ctx, request))
	other, err := s.postgresRepo.GetUserByUsername(s.ctx, request.Username)
	s.Require().NoError(err)

	for _, multiplayer := range []bool{false, true} {
		initialize := s.postgresRepo.InitializeSingleplayerScoring
		table := "singleplayer_game"

		var id uuid.UUID

		if multiplayer {
			table = "multiplayer_game"
			initialize = s.postgresRepo.InitializeMultiplayerScoring
			id, err = s.postgresRepo.NewMultiplayerGame(s.ctx, dto.NewMultiplayerGameRequest{
				CreatorID: owner.ID, ConnectedPlayers: []user.PublicProfile{owner.PublicProfile, other.PublicProfile},
				Provider: "google", Rounds: 2, RequestTime: fixtureTime(),
			})
		} else {
			id, err = s.postgresRepo.NewSingleplayerGame(s.ctx, dto.NewSingleplayerGameRequest{
				UserID: owner.ID, Provider: "google", Rounds: 2, RequestTime: fixtureTime(),
			})
		}

		s.Require().NoError(err)

		settings := game.ProviderScoring(game.GoogleProvider)
		s.Require().NoError(initialize(s.ctx, id, settings))
		s.Require().ErrorIs(initialize(s.ctx, id, game.ProviderScoring(game.SeznamProvider)), game.ErrInvalidState)

		var stored game.ScoringSettings

		err = s.pool.QueryRow(s.ctx,
			"SELECT scoring_version,scoring_scale_meters FROM "+table+" WHERE id=$1", id).
			Scan(&stored.Version, &stored.ScaleMeters)
		s.Require().NoError(err)
		s.Equal(settings, stored)
		_, err = s.pool.Exec(s.ctx, "UPDATE "+table+
			" SET finished=TRUE,scoring_version=NULL,scoring_scale_meters=NULL WHERE id=$1", id)
		s.Require().NoError(err)
		s.Require().ErrorIs(initialize(s.ctx, id, settings), game.ErrInvalidState)
		s.Require().ErrorIs(initialize(s.ctx, id, game.ScoringSettings{}), game.ErrInvalidScoring)
		_, err = s.pool.Exec(s.ctx, "UPDATE "+table+" SET scoring_scale_meters=1 WHERE id=$1", id)
		s.Require().Error(err)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	_, err = s.postgresRepo.GetPlayAreaPresetBounds(ctx, game.GoogleProvider, []uuid.UUID{uuid.New()})
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *ScoringTestSuite) TestPresetBounds() {
	for _, tc := range []struct {
		name     string
		geometry string
		want     game.GeographicBounds
	}{
		{
			name: "ordinary", geometry: "POLYGON((20 10,22 10,22 12,20 12,20 10))",
			want: game.GeographicBounds{MinLng: 20, MinLat: 10, MaxLng: 22, MaxLat: 12},
		},
		{
			name: "antimeridian", geometry: "MULTIPOLYGON(((179 60,180 60,180 61,179 61,179 60))," +
				"((-180 60,-179 60,-179 61,-180 61,-180 60)))",
			want: game.GeographicBounds{MinLng: 179, MinLat: 60, MaxLng: 181, MaxLat: 61},
		},
		{
			name: "greenwich", geometry: "POLYGON((-1 50,1 50,1 51,-1 51,-1 50))",
			want: game.GeographicBounds{MinLng: -1, MinLat: 50, MaxLng: 1, MaxLat: 51},
		},
	} {
		s.Run(tc.name, func() {
			id := s.addScoringPreset(tc.geometry)
			bounds, err := s.postgresRepo.GetPlayAreaPresetBounds(s.ctx, game.GoogleProvider, []uuid.UUID{id, id})
			s.Require().NoError(err)
			s.Equal(tc.want, bounds)
			_, err = s.postgresRepo.GetPlayAreaPresetBounds(s.ctx, game.YandexProvider, []uuid.UUID{id})
			s.Require().ErrorIs(err, playarea.ErrInvalidSelection)
		})
	}

	first := s.addScoringPreset("POLYGON((20 10,22 10,22 12,20 12,20 10))")
	second := s.addScoringPreset("POLYGON((21 11,23 11,23 13,21 13,21 11))")
	far := s.addScoringPreset("POLYGON((30 20,32 20,32 22,30 22,30 20))")
	bounds, err := s.postgresRepo.GetPlayAreaPresetBounds(s.ctx, game.GoogleProvider, []uuid.UUID{first, second, far})
	s.Require().NoError(err)
	s.Equal(game.GeographicBounds{MinLng: 20, MinLat: 10, MaxLng: 32, MaxLat: 22}, bounds)
	_, err = s.postgresRepo.GetPlayAreaPresetBounds(s.ctx, game.GoogleProvider, []uuid.UUID{first, uuid.New()})
	s.Require().ErrorIs(err, playarea.ErrInvalidSelection)
}

func (s *ScoringTestSuite) addScoringPreset(geometry string) uuid.UUID {
	s.T().Helper()

	id := uuid.New()
	query := `INSERT INTO play_area_preset
		(id,name,kind,country_code,version,source,license,attribution,providers,geom,display_geom)
		SELECT $1,'Scoring area','country','TST','test-v1','test','test','test',ARRAY['google'],geom,geom
		FROM (SELECT ST_Multi(ST_GeomFromText($2,4326)) AS geom) shape`
	_, err := s.pool.Exec(s.ctx, query, id, geometry)
	s.Require().NoError(err)

	return id
}

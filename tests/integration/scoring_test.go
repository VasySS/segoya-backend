//go:build integration

//nolint:paralleltest // Shared fixtures exercise sequential rounds and concurrent guesses explicitly.
package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	mp "github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	sp "github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	multiplayeruc "github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama"
	playareauc "github.com/VasySS/segoya-backend/internal/usecase/playarea"
	singleplayeruc "github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
)

type scoringFixture struct {
	t       *testing.T
	pool    *pgxpool.Pool
	repo    *postgres.Repository
	creator user.PrivateProfile
	other   user.PrivateProfile
	areas   *playareauc.Usecase
	polygon playarea.Polygon
}

type scoringGame struct {
	id           uuid.UUID
	table        string
	roundsTable  string
	guessesTable string
	submit       func(uuid.UUID, *game.LatLng) (int, error)
	next         func() error
}

func newScoringFixture(t *testing.T) scoringFixture {
	t.Helper()

	pool, repo := postgresFixture(t)
	width := 12_000 / game.EarthRadiusMeters * 180 / math.Pi
	height := 16_000 / game.EarthRadiusMeters * 180 / math.Pi
	_, err := pool.Exec(t.Context(), `INSERT INTO panorama_location(provider,lat,lng)
		VALUES ('google',0,0),('google',0,0.01)`)
	require.NoError(t, err)

	return scoringFixture{
		t: t, pool: pool, repo: repo, creator: createUser(t, repo), other: createUser(t, repo),
		areas: playareauc.NewUsecase(repo),
		polygon: playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{
			{{0, -height / 2}, {width, -height / 2}, {width, height / 2}, {0, height / 2}, {0, -height / 2}},
		}},
	}
}

func (f scoringFixture) newGame(multiplayer bool, selection playarea.Selection) scoringGame {
	f.t.Helper()

	panoramas := panorama.NewUsecase(panorama.Config{}, f.repo)
	result := scoringGame{
		table:        "singleplayer_game",
		roundsTable:  "singleplayer_round",
		guessesTable: "singleplayer_round_guess",
	}

	var err error

	if multiplayer {
		result.table, result.roundsTable = "multiplayer_game", "multiplayer_round"
		result.guessesTable = "multiplayer_round_user"
		uc := multiplayeruc.NewUsecase(multiplayeruc.Config{}, f.repo, panoramas, f.areas)
		result.id, err = uc.NewGame(f.t.Context(), dto.NewMultiplayerGameRequest{
			CreatorID: f.creator.ID, ConnectedPlayers: []user.PublicProfile{f.creator.PublicProfile, f.other.PublicProfile},
			Provider: "google", Rounds: 2, RequestTime: integrationTime(), PlayArea: selection,
		})
		result.submit = func(userID uuid.UUID, guess *game.LatLng) (int, error) {
			if err := uc.NewRoundGuess(f.t.Context(), dto.NewMultiplayerRoundGuessRequest{
				GameID: result.id, UserID: userID, Guess: guess, RequestTime: integrationTime().Add(time.Minute),
			}); err != nil {
				return 0, fmt.Errorf("submit multiplayer scoring guess: %w", err)
			}

			var points int

			err := f.pool.QueryRow(f.t.Context(), `SELECT score FROM multiplayer_round_user
				WHERE user_id=$2 AND round_id=(SELECT id FROM multiplayer_round WHERE game_id=$1
				ORDER BY round_num DESC LIMIT 1)`, result.id, userID).Scan(&points)
			if err != nil {
				return 0, fmt.Errorf("read multiplayer score: %w", err)
			}

			return points, nil
		}
		result.next = func() error {
			_, err := uc.NewRound(f.t.Context(), dto.NewMultiplayerRoundRequest{
				GameID: result.id, UserID: f.creator.ID, RequestTime: integrationTime().Add(time.Minute),
			})
			if err != nil {
				return fmt.Errorf("advance multiplayer scoring round: %w", err)
			}

			return nil
		}

		require.NoError(f.t, err)

		return result
	}

	uc := singleplayeruc.NewUsecase(singleplayeruc.Config{}, f.repo, panoramas, f.areas)
	result.id, err = uc.NewGame(f.t.Context(), dto.NewSingleplayerGameRequest{
		UserID: f.creator.ID, Provider: "google", Rounds: 2, RequestTime: integrationTime(), PlayArea: selection,
	})
	result.submit = func(userID uuid.UUID, guess *game.LatLng) (int, error) {
		response, err := uc.EndRound(f.t.Context(), dto.EndSingleplayerRoundRequest{
			GameID: result.id, UserID: userID, Guess: guess, RequestTime: integrationTime().Add(time.Minute),
		})
		if err != nil {
			return 0, fmt.Errorf("submit singleplayer scoring guess: %w", err)
		}

		return response.Score, nil
	}
	result.next = func() error {
		_, err := uc.NewRound(f.t.Context(), dto.NewSingleplayerRoundRequest{
			GameID: result.id, UserID: f.creator.ID, RequestTime: integrationTime().Add(time.Minute),
		})
		if err != nil {
			return fmt.Errorf("advance singleplayer scoring round: %w", err)
		}

		return nil
	}

	require.NoError(f.t, err)

	return result
}

func (f scoringFixture) scoring(g scoringGame) *game.ScoringSettings {
	f.t.Helper()

	if g.table == "multiplayer_game" {
		stored, err := f.repo.GetMultiplayerGame(f.t.Context(), g.id)
		require.NoError(f.t, err)

		return stored.Scoring
	}

	stored, err := f.repo.GetSingleplayerGame(f.t.Context(), g.id)
	require.NoError(f.t, err)

	return stored.Scoring
}

func (f scoringFixture) guess(g scoringGame) *game.LatLng {
	f.t.Helper()

	var location game.LatLng

	err := f.pool.QueryRow(f.t.Context(), `SELECT pl.lat,pl.lng FROM `+g.roundsTable+` r
		JOIN panorama_location pl ON pl.id=r.location_id WHERE r.game_id=$1
		ORDER BY r.round_num DESC LIMIT 1`, g.id).Scan(&location.Lat, &location.Lng)
	require.NoError(f.t, err)

	location.Lat += 1000 / game.EarthRadiusMeters * 180 / math.Pi

	return &location
}

func (f scoringFixture) finishRound(g scoringGame) {
	f.t.Helper()

	_, err := f.pool.Exec(f.t.Context(), "UPDATE "+g.roundsTable+" SET finished=TRUE,ended_at=$2 WHERE game_id=$1",
		g.id, integrationTime().Add(time.Second))
	require.NoError(f.t, err)
}

func TestGameScoringPersistenceAndLegacyInitialization(t *testing.T) {
	f := newScoringFixture(t)

	for _, multiplayer := range []bool{false, true} {
		t.Run(fmt.Sprintf("multiplayer=%t", multiplayer), func(t *testing.T) {
			saved, err := f.areas.Create(t.Context(), f.creator.ID, "City", f.polygon, "private")
			require.NoError(t, err)

			g := f.newGame(multiplayer, playarea.Selection{Type: "saved", SavedID: saved.ID})
			frozen := f.scoring(g)
			require.NotNil(t, frozen)
			require.Equal(t, game.ScoringVersionArea, frozen.Version)

			changed := playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {10, 0}, {0, 10}, {0, 0}}}}
			_, err = f.areas.Update(t.Context(), saved.ID, f.creator.ID, nil, &changed, nil)
			require.NoError(t, err)
			require.NoError(t, f.areas.Delete(t.Context(), saved.ID, f.creator.ID))
			points, err := g.submit(f.creator.ID, f.guess(g))
			require.NoError(t, err)
			require.InDelta(t, 4000, points, 1)
			require.Equal(t, frozen, f.scoring(g))

			legacy := f.newGame(multiplayer, playarea.Selection{Type: "polygon", Polygon: &f.polygon})
			providerScoring := game.ProviderScoring(game.GoogleProvider)
			_, err = f.pool.Exec(t.Context(), "UPDATE "+legacy.table+
				" SET scoring_version=$2,scoring_scale_meters=$3 WHERE id=$1",
				legacy.id, providerScoring.Version, providerScoring.ScaleMeters)
			require.NoError(t, err)
			oldPoints, err := legacy.submit(f.creator.ID, f.guess(legacy))
			require.NoError(t, err)
			require.GreaterOrEqual(t, oldPoints, 4999)
			f.finishRound(legacy)
			require.NoError(t, legacy.next())
			_, err = f.pool.Exec(t.Context(), "UPDATE "+legacy.table+
				" SET scoring_version=NULL,scoring_scale_meters=NULL WHERE id=$1", legacy.id)
			require.NoError(t, err)
			points, err = legacy.submit(f.creator.ID, f.guess(legacy))
			require.NoError(t, err)
			require.InDelta(t, 4000, points, 1)
			require.Equal(t, frozen, f.scoring(legacy))

			var historicalPoints int

			err = f.pool.QueryRow(t.Context(), "SELECT guess.score FROM "+legacy.guessesTable+" guess JOIN "+
				legacy.roundsTable+" r ON r.id=guess.round_id WHERE r.game_id=$1 AND r.round_num=1", legacy.id).
				Scan(&historicalPoints)
			require.NoError(t, err)
			require.Equal(t, oldPoints, historicalPoints)

			all := f.newGame(multiplayer, playarea.Selection{})
			require.Equal(t, game.ProviderScoring(game.GoogleProvider), *f.scoring(all))
			points, err = all.submit(f.creator.ID, f.guess(all))
			require.NoError(t, err)
			require.GreaterOrEqual(t, points, 4999)
		})
	}
}

func TestConcurrentFirstScoringGuesses(t *testing.T) {
	f := newScoringFixture(t)

	for _, multiplayer := range []bool{false, true} {
		t.Run(fmt.Sprintf("multiplayer=%t", multiplayer), func(t *testing.T) {
			g := f.newGame(multiplayer, playarea.Selection{Type: "polygon", Polygon: &f.polygon})
			_, err := f.pool.Exec(t.Context(), "UPDATE "+g.table+
				" SET scoring_version=NULL,scoring_scale_meters=NULL WHERE id=$1", g.id)
			require.NoError(t, err)

			guess := f.guess(g)

			users := []uuid.UUID{f.creator.ID, f.creator.ID}
			if multiplayer {
				users[1] = f.other.ID
			}

			type result struct {
				points int
				err    error
			}

			results := make(chan result, len(users))

			var workers sync.WaitGroup
			for _, userID := range users {
				workers.Go(func() { points, err := g.submit(userID, guess); results <- result{points: points, err: err} })
			}

			workers.Wait()
			close(results)

			successes := 0

			for result := range results {
				if !multiplayer && errors.Is(result.err, sp.ErrRoundAlreadyFinished) {
					continue
				}

				require.NoError(t, result.err)
				require.InDelta(t, 4000, result.points, 1)

				successes++
			}

			want := 1
			if multiplayer {
				want = 2
			}

			require.Equal(t, want, successes)
			require.Equal(t, game.ScoringVersionArea, f.scoring(g).Version)
		})
	}
}

type failingScoringRepository struct{ *postgres.Repository }

var errScoringQuery = errors.New("scoring boundary query failed")

func (r failingScoringRepository) GetPlayAreaPresetBounds(
	_ context.Context, _ game.PanoramaProvider, _ []uuid.UUID,
) (game.GeographicBounds, error) {
	return game.GeographicBounds{}, errScoringQuery
}

func TestScoringFailureAndCompletedGamesDoNotMutate(t *testing.T) {
	f := newScoringFixture(t)

	f.areas = playareauc.NewUsecase(failingScoringRepository{Repository: f.repo})

	for _, multiplayer := range []bool{false, true} {
		g := f.newGame(multiplayer, playarea.Selection{Type: "polygon", Polygon: &f.polygon})
		_, err := f.pool.Exec(t.Context(), "UPDATE "+g.table+
			" SET scoring_version=NULL,scoring_scale_meters=NULL,play_area_polygon=NULL,play_area_preset_ids=$2 WHERE id=$1",
			g.id, []uuid.UUID{uuid.New()})
		require.NoError(t, err)
		_, err = g.submit(f.creator.ID, f.guess(g))
		require.ErrorIs(t, err, errScoringQuery)
		require.Nil(t, f.scoring(g))

		var guesses int

		require.NoError(t, f.pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM "+g.guessesTable+
			" WHERE round_id IN (SELECT id FROM "+g.roundsTable+" WHERE game_id=$1)", g.id).Scan(&guesses))
		require.Zero(t, guesses)

		f.finishRound(g)
		_, err = f.pool.Exec(t.Context(), "UPDATE "+g.table+" SET finished=TRUE,rounds=1 WHERE id=$1", g.id)
		require.NoError(t, err)
		_, err = g.submit(f.creator.ID, f.guess(g))
		require.Error(t, err)
		require.Nil(t, f.scoring(g))
	}
}

func TestScoringSettingsStayInternal(t *testing.T) {
	t.Parallel()

	settings := game.ProviderScoring(game.GoogleProvider)
	entity := sp.Game{Scoring: &settings}

	multiplayer := mp.Game{Scoring: &settings}

	for _, value := range []any{
		entity,
		dto.SingleplayerGameToAPI(entity),
		multiplayer,
		dto.MultiplayerGameToAPI(multiplayer),
	} {
		content, err := json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(content), "scoring")
		require.NotContains(t, string(content), "scaleMeters")
	}
}

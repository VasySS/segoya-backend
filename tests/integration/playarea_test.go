//go:build integration

//nolint:paralleltest // Scenarios share one disposable database and deliberately synchronize game locks.
package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	multiplayeruc "github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama"
	playareauc "github.com/VasySS/segoya-backend/internal/usecase/playarea"
	singleplayeruc "github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
	"github.com/VasySS/segoya-backend/migrations/data"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func TestPlayAreaGameLifecycle(t *testing.T) {
	pool, repo := postgresFixture(t)
	creator, other := createUser(t, repo), createUser(t, repo)
	_, err := pool.Exec(t.Context(), `INSERT INTO panorama_location (provider,lat,lng) VALUES
		('google',1,1),('google',1,2),('google',2,1),('google',10,10),('yandex',1,1)`)
	require.NoError(t, err)

	polygon := playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {3, 0}, {3, 3}, {0, 3}, {0, 0}}}}
	areas := playareauc.NewUsecase(repo)
	panoramas := panorama.NewUsecase(panorama.Config{}, repo)

	for _, multiplayer := range []bool{false, true} {
		t.Run(fmt.Sprintf("multiplayer=%t", multiplayer), func(t *testing.T) {
			saved, err := areas.Create(t.Context(), creator.ID, "Original area", polygon, "private")
			require.NoError(t, err)

			var (
				gameID    uuid.UUID
				newGame   func(int, playarea.Selection) (uuid.UUID, error)
				nextRound func() (uuid.UUID, error)
			)

			table, roundsTable := "singleplayer_game", "singleplayer_round"
			if multiplayer {
				table, roundsTable = "multiplayer_game", "multiplayer_round"
				uc := multiplayeruc.NewUsecase(multiplayeruc.Config{}, repo, panoramas, areas)
				newGame = func(rounds int, selection playarea.Selection) (uuid.UUID, error) {
					return uc.NewGame(t.Context(), dto.NewMultiplayerGameRequest{
						CreatorID: creator.ID, ConnectedPlayers: []user.PublicProfile{creator.PublicProfile, other.PublicProfile},
						Provider: "google", Rounds: rounds, RequestTime: integrationTime(), PlayArea: selection,
					})
				}
				nextRound = func() (uuid.UUID, error) {
					round, err := uc.NewRound(t.Context(), dto.NewMultiplayerRoundRequest{
						GameID: gameID, UserID: creator.ID, RequestTime: integrationTime().Add(time.Minute),
					})
					if err != nil {
						return uuid.Nil, fmt.Errorf("request multiplayer round: %w", err)
					}

					return round.ID, nil
				}
			} else {
				uc := singleplayeruc.NewUsecase(singleplayeruc.Config{}, repo, panoramas, areas)
				newGame = func(rounds int, selection playarea.Selection) (uuid.UUID, error) {
					return uc.NewGame(t.Context(), dto.NewSingleplayerGameRequest{
						UserID: creator.ID, Provider: "google", Rounds: rounds, RequestTime: integrationTime(), PlayArea: selection,
					})
				}
				nextRound = func() (uuid.UUID, error) {
					round, err := uc.NewRound(t.Context(), dto.NewSingleplayerRoundRequest{
						GameID: gameID, UserID: creator.ID, RequestTime: integrationTime().Add(time.Minute),
					})
					if err != nil {
						return uuid.Nil, fmt.Errorf("request singleplayer round: %w", err)
					}

					return round.ID, nil
				}
			}

			gameID, err = newGame(3, playarea.Selection{Type: "saved", SavedID: saved.ID})
			require.NoError(t, err)

			changed := playarea.Polygon{
				Type:        "Polygon",
				Coordinates: [][][]float64{{{9, 9}, {11, 9}, {11, 11}, {9, 11}, {9, 9}}},
			}
			_, err = areas.Update(t.Context(), saved.ID, creator.ID, nil, &changed, nil)
			require.NoError(t, err)
			require.NoError(t, areas.Delete(t.Context(), saved.ID, creator.ID))

			var stored playarea.Selection

			require.NoError(t, pool.QueryRow(t.Context(), "SELECT play_area FROM "+table+" WHERE id=$1", gameID).Scan(&stored))
			require.Equal(t, playarea.Selection{Type: "polygon", Polygon: &polygon}, stored)

			finish := func() {
				_, err := pool.Exec(t.Context(), "UPDATE "+roundsTable+" SET finished=TRUE, ended_at=$2 WHERE game_id=$1",
					gameID, integrationTime().Add(time.Second))
				require.NoError(t, err)
			}
			finish()

			type result struct {
				id  uuid.UUID
				err error
			}

			results := make(chan result, 8)

			var workers sync.WaitGroup
			for range cap(results) {
				workers.Go(func() { id, err := nextRound(); results <- result{id: id, err: err} })
			}

			workers.Wait()
			close(results)

			ids := map[uuid.UUID]bool{}

			for result := range results {
				require.NoError(t, result.err)

				ids[result.id] = true
			}

			require.Len(t, ids, 1, "simultaneous requests must reuse the same second round")
			finish()

			_, err = nextRound()
			require.NoError(t, err)

			geometry, err := json.Marshal(polygon)
			require.NoError(t, err)

			var (
				total, distinct int
				inside          bool
			)

			require.NoError(t, pool.QueryRow(t.Context(), `SELECT COUNT(*), COUNT(DISTINCT r.location_id),
				bool_and(ST_Covers(ST_GeomFromGeoJSON($2::text),pl.geom))
				FROM `+roundsTable+` r JOIN panorama_location pl ON pl.id=r.location_id WHERE r.game_id=$1`,
				gameID, string(geometry)).Scan(&total, &distinct, &inside))
			require.Equal(t, 3, total)
			require.Equal(t, 3, distinct)
			require.True(t, inside)

			_, err = newGame(4, playarea.Selection{Type: "polygon", Polygon: &polygon})
			require.ErrorIs(t, err, playarea.ErrInsufficientLocations)
		})
	}
}

func TestPlayAreaQueryPlanWithSeededLocations(t *testing.T) {
	pool, repo := postgresFixture(t)
	require.NoError(t, data.MigrateAllCSVData(t.Context(), pool))
	_, err := pool.Exec(t.Context(), "ANALYZE panorama_location")
	require.NoError(t, err)

	polygon := playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{
		{14.40, 50.08}, {14.41, 50.08}, {14.41, 50.09}, {14.40, 50.09}, {14.40, 50.08},
	}}}
	started := time.Now()
	count, err := repo.CountPlayAreaLocations(t.Context(), game.SeznamProvider,
		playarea.Selection{Type: "polygon", Polygon: &polygon})
	require.NoError(t, err)
	t.Logf("count against CSV seed dataset: %d locations in %s", count, time.Since(started))

	geometry, err := json.Marshal(polygon)
	require.NoError(t, err)

	var plan json.RawMessage

	require.NoError(t, pool.QueryRow(t.Context(), `EXPLAIN (ANALYZE,FORMAT JSON)
		SELECT COUNT(*) FROM panorama_location pl WHERE provider='seznam'
		AND ST_Covers(ST_GeomFromGeoJSON($1::text),pl.geom)`, string(geometry)).Scan(&plan))
	require.Contains(t, string(plan), "panorama_location_geom_idx")
	t.Logf("spatial count query plan: %s", plan)
}

func TestPlayAreaMigrationOnRestoredCopy(t *testing.T) {
	container := containers.NewTestPostgres(t)
	pool := containers.NewTestPostgresPool(t, container.ConnectionString)
	require.NoError(t, tables.RunGooseMigrations(t.Context(), pool, "up-to", "11"))

	owner, locationID, gameID := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(t.Context(), `INSERT INTO user_info
		(id,username,name,password,register_date) VALUES ($1,'restore-user','Restore user','test',$2)`,
		owner, integrationTime())
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `INSERT INTO panorama_location(id,provider,lat,lng)
		VALUES ($1,'google',50.08,14.40)`, locationID)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `INSERT INTO singleplayer_game
		(id,user_id,rounds,provider,movement_allowed,timer_seconds,created_at)
		VALUES ($1,$2,1,'google',TRUE,0,$3)`, gameID, owner, integrationTime())
	require.NoError(t, err)

	configuration, err := pgxpool.ParseConfig(container.ConnectionString)
	require.NoError(t, err)

	run := func(command []string) {
		exit, output, err := container.Exec(t.Context(), command)
		require.NoError(t, err)
		message, err := io.ReadAll(output)
		require.NoError(t, err)
		require.Zero(t, exit, string(message))
	}
	run([]string{
		"pg_dump", "-U", configuration.ConnConfig.User, "-d", configuration.ConnConfig.Database,
		"-Fc", "-f", "/tmp/play-area-before.dump",
	})

	_, err = pool.Exec(t.Context(), "CREATE DATABASE play_area_restore")
	require.NoError(t, err)
	run([]string{
		"pg_restore", "-U", configuration.ConnConfig.User, "-d", "play_area_restore", "/tmp/play-area-before.dump",
	})

	configuration.ConnConfig.Database = "play_area_restore"
	restored, err := pgxpool.NewWithConfig(t.Context(), configuration)
	require.NoError(t, err)
	t.Cleanup(restored.Close)
	require.NoError(t, tables.RunGooseMigrations(t.Context(), restored, "up"))

	var selection playarea.Selection

	err = restored.QueryRow(t.Context(), "SELECT play_area FROM singleplayer_game WHERE id=$1", gameID).Scan(&selection)
	require.NoError(t, err)
	require.Equal(t, "all", selection.Type)

	var correctPoint bool

	require.NoError(t, restored.QueryRow(t.Context(), `SELECT ST_Equals(geom,ST_SetSRID(ST_MakePoint(lng,lat),4326))
		FROM panorama_location WHERE id=$1`, locationID).Scan(&correctPoint))
	require.True(t, correctPoint)

	var originalColumns int

	require.NoError(t, pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name='singleplayer_game' AND column_name='play_area'`).Scan(&originalColumns))
	require.Zero(t, originalColumns, "upgrading the restored copy must not alter the original database")
}

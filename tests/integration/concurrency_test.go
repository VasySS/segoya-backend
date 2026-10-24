//go:build integration

//nolint:paralleltest // Container scenarios run sequentially to isolate lock barriers and lifecycle assertions.
package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	multiplayeruc "github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	useruc "github.com/VasySS/segoya-backend/internal/usecase/user"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func postgresFixture(t *testing.T) (*pgxpool.Pool, *postgres.Repository) {
	t.Helper()
	container := containers.NewTestPostgres(t)
	pool := containers.NewTestPostgresPool(t, container.ConnectionString)
	require.NoError(t, tables.RunGooseMigrations(containers.TestContext(t), pool, "up"))

	return pool, postgres.New(postgres.NewTxManager(pool))
}

func createUser(t *testing.T, repo *postgres.Repository) user.PrivateProfile {
	t.Helper()

	name := uuid.NewString()
	require.NoError(t, repo.NewUser(t.Context(), dto.RegisterRequestDB{
		Username: name, Name: name, Password: "test", RequestTime: integrationTime(),
	}))

	profile, err := repo.GetUserByUsername(t.Context(), name)
	require.NoError(t, err)

	return profile
}

type gameGate struct {
	*postgres.Repository

	entered chan struct{}
	locked  chan struct{}
	release chan struct{}
}

func (r gameGate) LockMultiplayerGame(ctx context.Context, id uuid.UUID) error {
	close(r.entered)

	if err := r.Repository.LockMultiplayerGame(ctx, id); err != nil {
		return fmt.Errorf("lock game for barrier: %w", err)
	}

	close(r.locked)

	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for test gate: %w", ctx.Err())
	}
}

func TestGuessCompletionSerialization(t *testing.T) {
	pool, repo := postgresFixture(t)
	creator, other := createUser(t, repo), createUser(t, repo)

	locationID := createZeroLocation(t, pool)

	for _, guessFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "guess locks first", false: "completion locks first"}[guessFirst], func(t *testing.T) {
			workers := newTestWorkers(t)
			ctx := workers.ctx

			start := integrationTime().Add(-time.Minute).Truncate(time.Microsecond)
			gameID, err := repo.NewMultiplayerGame(ctx, dto.NewMultiplayerGameRequest{
				RequestTime: start, CreatorID: creator.ID,
				ConnectedPlayers: []user.PublicProfile{creator.PublicProfile, other.PublicProfile},
				Provider:         "google", Rounds: 1, TimerSeconds: 10,
			})
			require.NoError(t, err)
			_, err = repo.NewMultiplayerRound(ctx, dto.NewMultiplayerRoundRequestDB{
				GameID: gameID, LocationID: locationID, RoundNum: 1, CreatedAt: start, StartedAt: start,
			})
			require.NoError(t, err)

			first := gameGate{
				Repository: repo, entered: make(chan struct{}), locked: make(chan struct{}), release: make(chan struct{}),
			}
			second := gameGate{
				Repository: repo, entered: make(chan struct{}), locked: make(chan struct{}), release: make(chan struct{}),
			}
			close(second.release)

			guessRepo, endRepo := first, second
			if !guessFirst {
				guessRepo, endRepo = second, first
			}

			guessUC := multiplayeruc.NewUsecase(multiplayeruc.Config{}, guessRepo, nil)
			endUC := multiplayeruc.NewUsecase(multiplayeruc.Config{}, endRepo, nil)
			guessResult := make(chan error, 1)

			type completion struct {
				guesses []multiplayer.Guess
				err     error
			}

			endResult := make(chan completion, 1)
			guess := func() {
				guessResult <- guessUC.NewRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequest{
					RequestTime: start.Add(11 * time.Second), GameID: gameID, UserID: creator.ID, Guess: &game.LatLng{},
				})
			}
			end := func() {
				guesses, err := endUC.EndRound(ctx, dto.EndMultiplayerRoundRequest{
					RequestTime: start.Add(11 * time.Second), GameID: gameID, UserID: creator.ID,
				})
				endResult <- completion{guesses: guesses, err: err}
			}

			if guessFirst {
				workers.Go(guess)
			} else {
				workers.Go(end)
			}

			select {
			case <-first.locked:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			if guessFirst {
				workers.Go(end)
			} else {
				workers.Go(guess)
			}

			select {
			case <-second.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			waitForDatabaseLock(t, ctx, pool)
			close(first.release)

			guessErr := receive(t, ctx, guessResult)
			ended := receive(t, ctx, endResult)
			require.NoError(t, ended.err)

			if guessFirst {
				require.NoError(t, guessErr)
				require.Len(t, ended.guesses, 1)
				require.Equal(t, 5000, ended.guesses[0].Score)
			} else {
				require.ErrorIs(t, guessErr, multiplayer.ErrRoundAlreadyFinished)
				require.Empty(t, ended.guesses)
			}

			history, err := repo.GetMultiplayerGameGuesses(ctx, gameID)
			require.NoError(t, err)
			require.ElementsMatch(t, ended.guesses, history)
		})
	}
}

func TestGuessPresenceAndParticipantAuthorization(t *testing.T) {
	pool, repo := postgresFixture(t)
	creator, other, outsider := createUser(t, repo), createUser(t, repo), createUser(t, repo)
	ctx := containers.TestContext(t)
	start := integrationTime().Truncate(time.Microsecond)

	locationID := createZeroLocation(t, pool)
	gameID, err := repo.NewMultiplayerGame(ctx, dto.NewMultiplayerGameRequest{
		RequestTime: start, CreatorID: creator.ID,
		ConnectedPlayers: []user.PublicProfile{creator.PublicProfile, other.PublicProfile},
		Provider:         "google", Rounds: 1, TimerSeconds: 10,
	})
	require.NoError(t, err)

	_, err = repo.NewMultiplayerRound(ctx, dto.NewMultiplayerRoundRequestDB{
		GameID: gameID, LocationID: locationID, RoundNum: 1, CreatedAt: start, StartedAt: start,
	})
	require.NoError(t, err)

	uc := multiplayeruc.NewUsecase(multiplayeruc.Config{}, repo, nil)
	_, err = uc.GetGameUser(ctx, outsider.ID, gameID)
	require.ErrorIs(t, err, multiplayer.ErrGameWrongUserID)
	require.ErrorIs(t, uc.NewRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequest{
		RequestTime: start.Add(-time.Nanosecond), GameID: gameID, UserID: creator.ID,
	}), game.ErrRoundNotStarted)
	require.NoError(t, uc.NewRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequest{
		RequestTime: start, GameID: gameID, UserID: creator.ID,
	}))

	round, err := repo.GetMultiplayerRound(ctx, gameID, 1)
	require.NoError(t, err)
	require.Equal(t, 1, round.GuessesCount)

	guesses, err := repo.GetMultiplayerGameGuesses(ctx, gameID)
	require.NoError(t, err)
	require.Len(t, guesses, 1)
	require.Equal(t, creator.Username, guesses[0].Username)
	require.Zero(t, guesses[0].Score)
	require.Zero(t, guesses[0].Lat)
	require.Zero(t, guesses[0].Lng)

	var distance int

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT distance_miss_meters FROM multiplayer_round_user WHERE round_id=$1 AND user_id=$2`,
		round.ID, creator.ID).Scan(&distance))
	require.Zero(t, distance)

	profile, err := uc.GetGameUser(ctx, creator.ID, gameID)
	require.NoError(t, err)
	require.True(t, profile.Guessed)
	profile, err = uc.GetGameUser(ctx, other.ID, gameID)
	require.NoError(t, err)
	require.False(t, profile.Guessed)
	require.NoError(t, uc.NewRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequest{
		RequestTime: start.Add(time.Second), GameID: gameID, UserID: other.ID, Guess: &game.LatLng{},
	}))

	profile, err = uc.GetGameUser(ctx, creator.ID, gameID)
	require.NoError(t, err)
	require.Zero(t, profile.Score)
	profile, err = uc.GetGameUser(ctx, other.ID, gameID)
	require.NoError(t, err)
	require.Equal(t, 5000, profile.Score)

	ended, err := uc.EndRound(ctx, dto.EndMultiplayerRoundRequest{
		RequestTime: start.Add(time.Second), GameID: gameID, UserID: creator.ID,
	})
	require.NoError(t, err) // Both attempts count, even before the timer deadline.
	require.Len(t, ended, 2)

	history, err := repo.GetMultiplayerGameGuesses(ctx, gameID)
	require.NoError(t, err)
	require.ElementsMatch(t, ended, history)
}

func TestCanceledGuessWaitingForGameLockDoesNotPersist(t *testing.T) {
	pool, repo := postgresFixture(t)
	creator, other := createUser(t, repo), createUser(t, repo)
	workers := newTestWorkers(t)
	ctx := workers.ctx
	start := integrationTime()
	gameID, err := repo.NewMultiplayerGame(ctx, dto.NewMultiplayerGameRequest{
		RequestTime: start, CreatorID: creator.ID, Provider: "google", Rounds: 1, TimerSeconds: 10,
		ConnectedPlayers: []user.PublicProfile{creator.PublicProfile, other.PublicProfile},
	})
	require.NoError(t, err)
	_, err = repo.NewMultiplayerRound(ctx, dto.NewMultiplayerRoundRequestDB{
		GameID: gameID, LocationID: createZeroLocation(t, pool), RoundNum: 1, CreatedAt: start, StartedAt: start,
	})
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	lockResult := make(chan error, 1)

	workers.Go(func() {
		lockResult <- repo.RunTx(ctx, func(txCtx context.Context) error {
			if err := repo.LockMultiplayerGame(txCtx, gameID); err != nil {
				return fmt.Errorf("hold game lock: %w", err)
			}

			close(locked)

			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return fmt.Errorf("hold game lock: %w", ctx.Err())
			}
		})
	})
	receive(t, ctx, locked)

	guessCtx, cancelGuess := context.WithCancel(ctx)
	defer cancelGuess()

	uc := multiplayeruc.NewUsecase(multiplayeruc.Config{}, repo, nil)
	guessResult := make(chan error, 1)
	request := dto.NewMultiplayerRoundGuessRequest{
		RequestTime: start, GameID: gameID, UserID: creator.ID, Guess: &game.LatLng{},
	}

	workers.Go(func() { guessResult <- uc.NewRoundGuess(guessCtx, request) })
	waitForDatabaseLock(t, ctx, pool)
	cancelGuess()
	require.ErrorIs(t, receive(t, ctx, guessResult), context.Canceled)
	close(release)
	require.NoError(t, receive(t, ctx, lockResult))
	stored, err := repo.GetMultiplayerRound(ctx, gameID, 1)
	require.NoError(t, err)
	require.Zero(t, stored.GuessesCount)
	// A fresh request succeeds after cancellation releases the failed transaction.
	require.NoError(t, uc.NewRoundGuess(ctx, request))
	stored, err = repo.GetMultiplayerRound(ctx, gameID, 1)
	require.NoError(t, err)
	require.Equal(t, 1, stored.GuessesCount)
}

type avatarStorage struct {
	uploads   atomic.Int32
	deletions atomic.Int32
	entered   chan struct{}
	release   chan struct{}
}

func (s *avatarStorage) UploadAvatar(ctx context.Context, _ io.Reader, _, _ string) error {
	s.uploads.Add(1)
	close(s.entered)

	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for test gate: %w", ctx.Err())
	}
}
func (s *avatarStorage) DeleteAvatar(context.Context, string) error { s.deletions.Add(1); return nil }

func TestAvatarCooldownSerialization(t *testing.T) {
	pool, repo := postgresFixture(t)
	profile := createUser(t, repo)

	workers := newTestWorkers(t)
	ctx := workers.ctx

	storage := &avatarStorage{entered: make(chan struct{}), release: make(chan struct{})}
	uc := useruc.NewUsecase(useruc.Config{AvatarUpdateLimit: time.Hour}, repo, storage)
	now := integrationTime().Truncate(time.Microsecond)

	first, second := make(chan error, 1), make(chan error, 1)

	workers.Go(func() {
		first <- uc.UpdateAvatar(ctx, dto.UpdateAvatarRequest{
			UserID: profile.ID, RequestTime: now, File: bytes.NewBufferString("first"), MimeType: "image/png",
		})
	})

	select {
	case <-storage.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	workers.Go(func() {
		second <- uc.UpdateAvatar(ctx, dto.UpdateAvatarRequest{
			UserID: profile.ID, RequestTime: now, File: bytes.NewBufferString("second"), MimeType: "image/png",
		})
	})

	waitForDatabaseLock(t, ctx, pool)
	close(storage.release)
	require.NoError(t, receive(t, ctx, first))
	require.ErrorIs(t, receive(t, ctx, second), user.ErrAvatarUpdateTooFrequent)
	require.EqualValues(t, 1, storage.uploads.Load())
	require.Zero(t, storage.deletions.Load())

	stored, err := repo.GetUserByID(ctx, profile.ID)
	require.NoError(t, err)
	require.Equal(t, now, stored.AvatarLastUpdate)
}

func createZeroLocation(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID

	query := `INSERT INTO panorama_location(provider,lat,lng) VALUES ('google',0,0) RETURNING id`
	require.NoError(t, pool.QueryRow(t.Context(), query).Scan(&id))

	return id
}

func waitForDatabaseLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	query := `SELECT EXISTS(
		SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND datname=current_database()
	)`

	require.Eventually(t, func() bool {
		var blocked bool

		err := pool.QueryRow(ctx, query).Scan(&blocked)

		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
}

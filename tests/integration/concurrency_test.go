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
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	valkeyrepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
	multiplayeruc "github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	useruc "github.com/VasySS/segoya-backend/internal/usecase/user"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func postgresFixture(t *testing.T) (*pgxpool.Pool, *postgres.Repository) {
	t.Helper()
	container, err := containers.NewPostgresContainer(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
		defer cancel()

		require.NoError(t, container.Terminate(ctx))
	})

	pool, err := pgxpool.New(t.Context(), container.ConnectionString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, tables.RunGooseMigrations(t.Context(), pool, "up"))

	return pool, postgres.New(postgres.NewTxManager(pool))
}

func createUser(t *testing.T, repo *postgres.Repository) user.PrivateProfile {
	t.Helper()

	name := uuid.NewString()
	require.NoError(t, repo.NewUser(t.Context(), dto.RegisterRequestDB{
		Username: name, Name: name, Password: "test", RequestTime: time.Now().UTC(),
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
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()

			start := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
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
				go guess()
			} else {
				go end()
			}

			select {
			case <-first.locked:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			if guessFirst {
				go end()
			} else {
				go guess()
			}

			select {
			case <-second.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			waitForDatabaseLock(t, ctx, pool)
			close(first.release)

			guessErr := <-guessResult
			ended := <-endResult
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
	ctx := t.Context()
	start := time.Now().UTC().Truncate(time.Microsecond)

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

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	storage := &avatarStorage{entered: make(chan struct{}), release: make(chan struct{})}
	uc := useruc.NewUsecase(useruc.Config{AvatarUpdateLimit: time.Hour}, repo, storage)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first, second := make(chan error, 1), make(chan error, 1)

	go func() {
		first <- uc.UpdateAvatar(ctx, dto.UpdateAvatarRequest{
			UserID: profile.ID, RequestTime: now, File: bytes.NewBufferString("first"), MimeType: "image/png",
		})
	}()

	select {
	case <-storage.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	go func() {
		second <- uc.UpdateAvatar(ctx, dto.UpdateAvatarRequest{
			UserID: profile.ID, RequestTime: now, File: bytes.NewBufferString("second"), MimeType: "image/png",
		})
	}()

	waitForDatabaseLock(t, ctx, pool)
	close(storage.release)
	require.NoError(t, <-first)
	require.ErrorIs(t, <-second, user.ErrAvatarUpdateTooFrequent)
	require.EqualValues(t, 1, storage.uploads.Load())
	require.Zero(t, storage.deletions.Load())

	stored, err := repo.GetUserByID(ctx, profile.ID)
	require.NoError(t, err)
	require.Equal(t, now, stored.AvatarLastUpdate)
}

func TestAtomicLobbyMembership(t *testing.T) {
	container, err := containers.NewValkeyContainer(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
		defer cancel()

		require.NoError(t, container.Terminate(ctx))
	})

	client, err := valkey.NewClient(valkey.MustParseURL(container.ConnectionString))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	repo := valkeyrepo.New(t.Context(), client)

	for _, private := range []bool{false, true} {
		t.Run(map[bool]string{false: "public", true: "private"}[private], func(t *testing.T) {
			id, creator, first, second := uuid.NewString(), uuid.New(), uuid.New(), uuid.New()

			req := dto.NewLobbyRequestDB{
				ID: id, CreatorID: creator, RequestTime: time.Now().UTC(), Provider: "google", Rounds: 1, MaxPlayers: 2,
			}
			if private {
				err = repo.NewPrivateLobby(t.Context(), req)
			} else {
				err = repo.NewLobby(t.Context(), req)
			}

			require.NoError(t, err)

			prefix := "lobby:"
			if private {
				prefix = "lobby:private:"
			}

			ttl := func() int64 {
				result, err := client.Do(t.Context(), client.B().Pttl().Key(prefix+id).Build()).AsInt64()
				require.NoError(t, err)

				return result
			}
			require.Positive(t, ttl())
			require.NoError(t, repo.AdmitLobbyUser(t.Context(), id, creator, "creator"))
			require.EqualValues(t, -1, ttl())

			require.NoError(t, repo.AdmitLobbyUser(t.Context(), id, creator, "creator"))
			require.ErrorIs(t, repo.AdmitLobbyUser(t.Context(), id, creator, "duplicate"), lobby.ErrAlreadyConnected)

			ready, results := make(chan struct{}), make(chan error, 2)

			for _, uid := range []uuid.UUID{first, second} {
				go func() { <-ready; results <- repo.AdmitLobbyUser(t.Context(), id, uid, uid.String()) }()
			}

			close(ready)

			joined, rejected := 0, 0

			for range 2 {
				if err := <-results; err == nil {
					joined++
				} else {
					require.ErrorIs(t, err, lobby.ErrLobbyIsFull)

					rejected++
				}
			}

			require.Equal(t, 1, joined)
			require.Equal(t, 1, rejected)
			value, err := repo.GetLobby(t.Context(), id)
			require.NoError(t, err)
			require.Equal(t, 2, value.CurrentPlayers)
			require.NoError(t, repo.ReleaseLobbyUser(t.Context(), id, creator, "wrong", 3*time.Minute))

			ready = make(chan struct{})

			for _, uid := range []uuid.UUID{creator, first, second} {
				go func() {
					<-ready

					sid := uid.String()
					if uid == creator {
						sid = "creator"
					}

					results <- repo.ReleaseLobbyUser(t.Context(), id, uid, sid, 3*time.Minute)
				}()
			}

			close(ready)

			for range 3 {
				require.NoError(t, <-results)
			}

			value, err = repo.GetLobby(t.Context(), id)
			require.NoError(t, err)
			require.Zero(t, value.CurrentPlayers)
			require.Positive(t, ttl())
			require.NoError(t, repo.ReleaseLobbyUser(t.Context(), id, creator, "creator", 3*time.Minute))
			require.NoError(t, repo.AdmitLobbyUser(t.Context(), id, creator, "new"))
			require.NoError(t, repo.ReleaseLobbyUser(t.Context(), id, creator, "creator", 3*time.Minute))
			value, err = repo.GetLobby(t.Context(), id)
			require.NoError(t, err)
			require.Equal(t, 1, value.CurrentPlayers)
			require.EqualValues(t, -1, ttl())
			raceLobbyJoinAndRelease(t, repo, id, creator, first, ttl)
		})
	}

	exerciseLobbyWatchRetries(t, client)
}

func raceLobbyJoinAndRelease(
	t *testing.T, repo *valkeyrepo.Repository, id string, creator, joining uuid.UUID, ttl func() int64,
) {
	t.Helper()

	results := make(chan error, 2)
	// Racing the last release with a new admission must leave one persistent member.
	for range 10 {
		race := make(chan struct{})
		go func() {
			<-race

			results <- repo.ReleaseLobbyUser(t.Context(), id, creator, "new", 3*time.Minute)
		}()
		go func() {
			<-race

			results <- repo.AdmitLobbyUser(t.Context(), id, joining, "joined")
		}()

		close(race)

		for range 2 {
			require.NoError(t, <-results)
		}

		value, err := repo.GetLobby(t.Context(), id)
		require.NoError(t, err)
		require.Equal(t, 1, value.CurrentPlayers)
		require.EqualValues(t, -1, ttl())
		require.NoError(t, repo.ReleaseLobbyUser(t.Context(), id, joining, "joined", 3*time.Minute))
		require.NoError(t, repo.AdmitLobbyUser(t.Context(), id, creator, "new"))
	}
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

package singleplayer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/singleplayer"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	entity "github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

type contextKey struct{}

type tokens struct{ authorized bool }

func (s tokens) FromContext(context.Context) (user.AccessTokenClaims, bool) {
	return user.AccessTokenClaims{UserID: playerID()}, s.authorized
}

// Each method checks the request at the use-case boundary. Unauthorized requests
// must never reach this stub; calls are checked separately by the handler tests.
type usecaseStub struct {
	t         *testing.T
	failure   error
	calls     int
	guess     *game.LatLng
	operation string
}

func (u *usecaseStub) NewGame(ctx context.Context, req dto.NewSingleplayerGameRequest) (uuid.UUID, error) {
	u.operation = "create game"
	u.request(ctx, gameID(), req.UserID, req.RequestTime)
	assert.Equal(u.t, "google", req.Provider)
	assert.Equal(u.t, 5, req.Rounds)
	assert.Equal(u.t, 30, req.TimerSeconds)
	assert.True(u.t, req.MovementAllowed)

	return gameID(), u.failure
}

func (u *usecaseStub) GetGame(ctx context.Context, req dto.GetSingleplayerGameRequest) (entity.Game, error) {
	u.operation = "get game"
	u.request(ctx, req.GameID, req.UserID, req.RequestTime)

	return entity.Game{ID: gameID(), UserID: playerID(), Provider: game.GoogleProvider, Rounds: 5, Score: 123}, u.failure
}

func (u *usecaseStub) GetGames(ctx context.Context, req dto.GetSingleplayerGamesRequest) ([]entity.Game, int, error) {
	u.operation = "list games"
	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(contextKey{}))
	assert.Equal(u.t, dto.GetSingleplayerGamesRequest{UserID: playerID(), Page: 2, PageSize: 3}, req)

	return []entity.Game{{ID: gameID(), UserID: playerID(), Provider: game.GoogleProvider}}, 7, u.failure
}

func (u *usecaseStub) EndGame(ctx context.Context, req dto.EndSingleplayerGameRequest) error {
	u.operation = "end game"
	u.request(ctx, req.GameID, req.UserID, req.RequestTime)

	return u.failure
}

func (u *usecaseStub) NewRound(ctx context.Context, req dto.NewSingleplayerRoundRequest) (entity.Round, error) {
	u.operation = "new round"
	u.request(ctx, req.GameID, req.UserID, req.RequestTime)

	return entity.Round{ID: roundID(), GameID: gameID(), RoundNum: 2, StreetviewID: "panorama"}, u.failure
}

func (u *usecaseStub) GetRound(ctx context.Context, req dto.GetSingleplayerRoundRequest) (entity.Round, error) {
	u.operation = "get round"
	u.request(ctx, req.GameID, req.UserID, req.RequestTime)

	return entity.Round{ID: roundID(), GameID: gameID(), RoundNum: 2, StreetviewID: "panorama"}, u.failure
}

func (u *usecaseStub) GetGameGuesses(
	ctx context.Context,
	req dto.GetSingleplayerGameGuessesRequest,
) ([]entity.Guess, error) {
	u.operation = "get guesses"
	u.request(ctx, req.GameID, req.UserID, req.RequestTime)

	return []entity.Guess{{RoundNum: 1, Score: 456, GuessLat: 12, GuessLng: 34}}, u.failure
}

func (u *usecaseStub) EndRound(
	ctx context.Context,
	req dto.EndSingleplayerRoundRequest,
) (dto.EndCurrentRoundResponse, error) {
	u.operation = "end round"
	u.request(ctx, req.GameID, req.UserID, req.RequestTime)
	assert.Equal(u.t, u.guess, req.Guess)

	return dto.EndCurrentRoundResponse{Score: 456, Distance: 789}, u.failure
}

func TestHandlerContracts(t *testing.T) {
	t.Parallel()

	type rejection struct {
		name   string
		cause  error
		want   any
		status int
	}

	operations := []struct {
		name         string
		invoke       func(context.Context, *singleplayer.Handler) (any, error)
		want         any
		unauthorized any
		unexpected   any
		rejections   []rejection
	}{
		{
			name: "create game",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.NewSingleplayerGame(ctx, &api.NewSingleplayerGameRequest{
					Provider: "google", Rounds: 5, TimerSeconds: api.NewOptInt(30), MovementAllowed: true,
				})
			},
			want:         &api.NewSingleplayerGameCreated{ID: gameID()},
			unauthorized: &api.NewSingleplayerGameUnauthorized{}, unexpected: &api.NewSingleplayerGameInternalServerError{},
		},
		{
			name: "get game",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.GetSingleplayerGame(ctx, api.GetSingleplayerGameParams{ID: gameID()})
			},
			want:         &api.SingleplayerGame{ID: gameID(), UserID: playerID(), Provider: "google", Rounds: 5, Score: 123},
			unauthorized: &api.GetSingleplayerGameUnauthorized{}, unexpected: &api.GetSingleplayerGameInternalServerError{},
			rejections: []rejection{
				{"missing game", entity.ErrGameNotFound, &api.GetSingleplayerGameNotFound{}, 404},
				{"other owner", entity.ErrGameWrongUserID, &api.GetSingleplayerGameForbidden{}, 403},
			},
		},
		{
			name: "list games",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.GetSingleplayerGames(ctx, api.GetSingleplayerGamesParams{Page: 2, PageSize: 3})
			},
			want: &api.SingleplayerGames{
				Total: 7,
				Games: []api.SingleplayerGame{{
					ID:       gameID(),
					UserID:   playerID(),
					Provider: "google",
				}},
			},

			unauthorized: &api.GetSingleplayerGamesUnauthorized{}, unexpected: &api.GetSingleplayerGamesInternalServerError{},
		},
		{
			name: "end game",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.EndSingleplayerGame(ctx, api.EndSingleplayerGameParams{ID: gameID()})
			},
			want:         &api.EndSingleplayerGameNoContent{},
			unauthorized: &api.EndSingleplayerGameUnauthorized{},
			unexpected:   &api.EndSingleplayerGameInternalServerError{},

			rejections: []rejection{
				{"missing game", entity.ErrGameNotFound, &api.EndSingleplayerGameNotFound{}, 404},
				{"other owner", entity.ErrGameWrongUserID, &api.EndSingleplayerGameForbidden{}, 403},
				{"active game", entity.ErrGameIsStillActive, &api.EndSingleplayerGameBadRequest{}, 400},
				{"active round", entity.ErrRoundIsStillActive, &api.EndSingleplayerGameBadRequest{}, 400},
			},
		},
		{
			name: "new round",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.NewSingleplayerRound(ctx, api.NewSingleplayerRoundParams{ID: gameID()})
			},
			want:         &api.SingleplayerRound{ID: roundID(), GameID: gameID(), RoundNum: 2, StreetviewID: "panorama"},
			unauthorized: &api.NewSingleplayerRoundUnauthorized{}, unexpected: &api.NewSingleplayerRoundInternalServerError{},
			rejections: []rejection{
				{"missing game", entity.ErrGameNotFound, &api.NewSingleplayerRoundNotFound{}, 404},
				{"other owner", entity.ErrGameWrongUserID, &api.NewSingleplayerRoundForbidden{}, 403},
				{"round limit", entity.ErrRoundMaxAmount, &api.NewSingleplayerRoundBadRequest{}, 400},
			},
		},
		{
			name: "get round",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.GetSingleplayerRound(ctx, api.GetSingleplayerRoundParams{ID: gameID()})
			},
			want:         &api.SingleplayerRound{ID: roundID(), GameID: gameID(), RoundNum: 2, StreetviewID: "panorama"},
			unauthorized: &api.GetSingleplayerRoundUnauthorized{}, unexpected: &api.GetSingleplayerRoundInternalServerError{},
			rejections: []rejection{
				{"missing game", entity.ErrGameNotFound, &api.GetSingleplayerRoundNotFound{}, 404},
				{"other owner", entity.ErrGameWrongUserID, &api.GetSingleplayerRoundForbidden{}, 403},
			},
		},
		{
			name: "end round",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.EndSingleplayerRound(ctx,
					&api.SingleplayerRoundGuess{Guess: api.NilLatLng{Null: true}},
					api.EndSingleplayerRoundParams{ID: gameID()})
			},
			want:         &api.EndSingleplayerRoundResponse{Score: 456, Distance: 789},
			unauthorized: &api.EndSingleplayerRoundUnauthorized{}, unexpected: &api.EndSingleplayerRoundInternalServerError{},
			rejections: []rejection{
				{"missing game", entity.ErrGameNotFound, &api.EndSingleplayerRoundNotFound{}, 404},
				{"other owner", entity.ErrGameWrongUserID, &api.EndSingleplayerRoundForbidden{}, 403},
				{"already finished", entity.ErrRoundAlreadyFinished, &api.EndSingleplayerRoundBadRequest{}, 400},
				{"before start", game.ErrRoundNotStarted, &api.EndSingleplayerRoundBadRequest{}, 400},
				{"invalid coordinates", game.ErrInvalidCoordinates, &api.EndSingleplayerRoundBadRequest{}, 400},
			},
		},
		{
			name: "get guesses",
			invoke: func(ctx context.Context, h *singleplayer.Handler) (any, error) {
				return h.GetSingleplayerGameGuesses(ctx, api.GetSingleplayerGameGuessesParams{ID: gameID()})
			},
			want: &api.GetSingleplayerGameGuessesOKApplicationJSON{{
				RoundNum: 1,
				Score:    456,
				GuessLat: 12,
				GuessLng: 34,
			}},

			unauthorized: &api.GetSingleplayerGameGuessesUnauthorized{},
			unexpected:   &api.GetSingleplayerGameGuessesInternalServerError{},

			rejections: []rejection{
				{"missing game", entity.ErrGameNotFound, &api.GetSingleplayerGameGuessesNotFound{}, 404},
				{"other owner", entity.ErrGameWrongUserID, &api.GetSingleplayerGameGuessesForbidden{}, 403},
				{"active game", entity.ErrGameIsStillActive, &api.GetSingleplayerGameGuessesBadRequest{}, 400},
			},
		},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()

			cases := append([]rejection{
				{
					name: "success",
					want: operation.want,
				},
				{
					"unexpected failure",
					errors.New("private database detail"),
					operation.unexpected,
					500,
				},
			},
				operation.rejections...)
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					uc := &usecaseStub{t: t}
					if tc.cause != nil {
						uc.failure = fmt.Errorf("wrapped: %w", tc.cause)
					}

					h := singleplayer.NewHandler(singleplayer.Config{}, uc, tokens{authorized: true})
					ctx := context.WithValue(t.Context(), contextKey{}, "metadata")
					result, err := operation.invoke(ctx, h)
					require.NoError(t, err)
					assert.Equal(t, 1, uc.calls)
					assert.Equal(t, operation.name, uc.operation)

					if tc.cause == nil {
						assert.Equal(t, tc.want, result)
						return
					}

					assert.IsType(t, tc.want, result)
					body, err := json.Marshal(result)
					require.NoError(t, err)

					var problem api.BackendError

					require.NoError(t, json.Unmarshal(body, &problem))
					assert.Equal(t, tc.status, problem.Status)
					assert.NotEmpty(t, problem.Title)
					assert.NotEmpty(t, problem.Detail)
					assert.NotContains(t, problem.Detail, "private database detail")
				})
			}

			t.Run("unauthorized", func(t *testing.T) {
				t.Parallel()
				uc := &usecaseStub{t: t}
				h := singleplayer.NewHandler(singleplayer.Config{}, uc, tokens{})
				result, err := operation.invoke(t.Context(), h)
				require.NoError(t, err)
				// Creation's problem-body status mismatch is tracked separately;
				// do not encode it as the desired authorization contract here.
				assert.IsType(t, operation.unauthorized, result)
				assert.Zero(t, uc.calls)
			})
		})
	}
}

func TestNewGameRejectsInvalidSettingsBeforeUsecase(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		provider      api.Provider
		rounds, timer int
	}{
		{"unknown provider", "unknown", 5, 30}, {"no rounds", "google", 0, 30}, {"short timer", "google", 5, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := &usecaseStub{t: t}
			h := singleplayer.NewHandler(singleplayer.Config{}, uc, tokens{authorized: true})
			result,
				err := h.NewSingleplayerGame(t.Context(),
				&api.NewSingleplayerGameRequest{
					Provider:     tc.provider,
					Rounds:       tc.rounds,
					TimerSeconds: api.NewOptInt(tc.timer),
				})
			require.NoError(t, err)
			assert.IsType(t, &api.NewSingleplayerGameBadRequest{}, result)
			assert.Zero(t, uc.calls)
		})
	}
}

func TestEndRoundPreservesZeroCoordinateGuess(t *testing.T) {
	t.Parallel()
	uc := &usecaseStub{t: t, guess: &game.LatLng{}}
	h := singleplayer.NewHandler(singleplayer.Config{}, uc, tokens{authorized: true})
	ctx := context.WithValue(t.Context(), contextKey{}, "metadata")
	result,
		err := h.EndSingleplayerRound(ctx,
		&api.SingleplayerRoundGuess{Guess: api.NilLatLng{Value: api.LatLng{}}},
		api.EndSingleplayerRoundParams{ID: gameID()})
	require.NoError(t, err)
	assert.Equal(t, &api.EndSingleplayerRoundResponse{Score: 456, Distance: 789}, result)
	assert.Equal(t, 1, uc.calls)
}

func playerID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000001") }

func gameID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000002") }

func roundID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000003") }

func (u *usecaseStub) request(ctx context.Context, id, uid uuid.UUID, at time.Time) {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(contextKey{}))
	assert.Equal(u.t, gameID(), id)
	assert.Equal(u.t, playerID(), uid)
	assert.WithinDuration(u.t, time.Now().UTC(), at, time.Second)
}

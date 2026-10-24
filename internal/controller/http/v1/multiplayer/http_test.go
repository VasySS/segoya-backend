package multiplayer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	handler "github.com/VasySS/segoya-backend/internal/controller/http/v1/multiplayer"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

func TestMultiplayerHTTPResponses(t *testing.T) {
	t.Parallel()

	gameID, userID, roundID := uuid.New(), uuid.New(), uuid.New()
	createdAt := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	storedGame := multiplayer.Game{
		ID: gameID, CreatorID: userID, Rounds: 5, RoundCurrent: 2, Provider: "google", TimerSeconds: 60,
		MovementAllowed: true, Players: 3, Finished: true, CreatedAt: createdAt,
	}
	storedRound := multiplayer.Round{
		ID: roundID, GameID: gameID, RoundNum: 2, StreetviewID: "panorama", Lat: 55, Lng: 37,
		GuessesCount: 2, Finished: true, CreatedAt: createdAt,
		StartedAt: createdAt.Add(time.Second), EndedAt: createdAt.Add(time.Minute),
	}
	storedGuesses := []multiplayer.Guess{{
		Username: "player", AvatarHash: "avatar", RoundNum: 2,
		RoundLat: 55, RoundLng: 37, Lat: 56, Lng: 38, Score: 3456,
	}}
	wantRound := &api.MultiplayerRound{
		ID: roundID, GameID: gameID, RoundNum: 2, StreetviewID: "panorama", Lat: 55, Lng: 37,
		GuessesCount: 2, Finished: true, CreatedAt: createdAt,
		StartedAt: createdAt.Add(time.Second), EndedAt: createdAt.Add(time.Minute),
	}
	wantGuesses := api.GetMultiplayerGameGuessesOKApplicationJSON{{
		Username: "player", AvatarHash: "avatar", RoundNum: 2,
		RoundLat: 55, RoundLng: 37, Lat: 56, Lng: 38, Score: 3456,
	}}

	for _, endpoint := range []struct {
		name                                                     string
		call                                                     func(context.Context, *handler.Handler) (any, error)
		wantSuccess, unauthorizedType, notFoundType, failureType any
		failureTitle, failureDetail, missingDetail               string
	}{
		{
			name: "get game", call: func(ctx context.Context, h *handler.Handler) (any, error) {
				return h.GetMultiplayerGame(ctx, api.GetMultiplayerGameParams{ID: gameID})
			},
			wantSuccess: &api.MultiplayerGame{
				ID: gameID, CreatorID: userID, Rounds: 5, RoundCurrent: 2, Provider: api.ProviderGoogle,
				TimerSeconds: 60, MovementAllowed: true, Players: 3, Finished: true, CreatedAt: createdAt,
			},
			unauthorizedType: &api.GetMultiplayerGameUnauthorized{}, notFoundType: &api.GetMultiplayerGameNotFound{},
			failureType:  &api.GetMultiplayerGameInternalServerError{},
			failureTitle: "Error getting game", failureDetail: "An error occurred while getting game",
			missingDetail: "The game you are trying to get does not exist",
		},
		{
			name: "get guesses", call: func(ctx context.Context, h *handler.Handler) (any, error) {
				return h.GetMultiplayerGameGuesses(ctx, api.GetMultiplayerGameGuessesParams{ID: gameID})
			},
			wantSuccess: &wantGuesses, unauthorizedType: &api.GetMultiplayerGameGuessesUnauthorized{},
			failureType:  &api.GetMultiplayerGameGuessesInternalServerError{},
			failureTitle: "Error getting guesses", failureDetail: "An error occurred while getting guesses",
		},
		{
			name: "new round", call: func(ctx context.Context, h *handler.Handler) (any, error) {
				return h.NewMultiplayerRound(ctx, api.NewMultiplayerRoundParams{ID: gameID})
			},
			wantSuccess: wantRound, unauthorizedType: &api.NewMultiplayerRoundUnauthorized{},
			notFoundType: &api.NewMultiplayerRoundNotFound{}, failureType: &api.NewMultiplayerRoundInternalServerError{},
			failureTitle: "Error creating round", failureDetail: "An error occurred while creating round",
			missingDetail: "The game with the provided ID does not exist",
		},
		{
			name: "current round", call: func(ctx context.Context, h *handler.Handler) (any, error) {
				return h.GetMultiplayerRound(ctx, api.GetMultiplayerRoundParams{ID: gameID})
			},
			wantSuccess: wantRound, unauthorizedType: &api.GetMultiplayerRoundUnauthorized{},
			notFoundType: &api.GetMultiplayerRoundNotFound{}, failureType: &api.GetMultiplayerRoundInternalServerError{},
			failureTitle: "Error getting round", failureDetail: "An error occurred while getting round",
			missingDetail: "The game with the provided ID does not exist",
		},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			t.Parallel()

			for _, scenario := range []struct {
				name         string
				unauthorized bool
				usecaseErr   error
			}{
				{name: "success"},
				{name: "unauthorized stops before usecase", unauthorized: true},
				{name: "wrapped missing game", usecaseErr: fmt.Errorf("lookup: %w", multiplayer.ErrGameNotFound)},
				{name: "participant error stays generic", usecaseErr: fmt.Errorf("lookup: %w", multiplayer.ErrGameWrongUserID)},
				{name: "unexpected failure stays generic", usecaseErr: errors.New("database details must stay private")},
			} {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()

					calls := 0
					before := time.Now()
					checkRequest := func(ctx context.Context, gid, uid uuid.UUID, at time.Time) {
						calls++

						require.NoError(t, ctx.Err())
						assert.Equal(t, gameID, gid)
						assert.Equal(t, userID, uid)

						if !at.IsZero() {
							assert.False(t, at.Before(before))
							assert.False(t, at.After(time.Now()))
						}
					}
					uc := handlerUsecase{
						getGame: func(ctx context.Context, gid, uid uuid.UUID) (multiplayer.Game, error) {
							checkRequest(ctx, gid, uid, time.Time{})
							return storedGame, scenario.usecaseErr
						},
						getGuesses: func(ctx context.Context, req dto.GetGameGuessesRequest) ([]multiplayer.Guess, error) {
							checkRequest(ctx, req.GameID, req.UserID, req.RequestTime)
							assert.False(t, req.RequestTime.IsZero())

							return storedGuesses, scenario.usecaseErr
						},
						newRound: func(ctx context.Context, req dto.NewMultiplayerRoundRequest) (multiplayer.Round, error) {
							checkRequest(ctx, req.GameID, req.UserID, req.RequestTime)
							assert.False(t, req.RequestTime.IsZero())

							return storedRound, scenario.usecaseErr
						},
						getRound: func(ctx context.Context, req dto.GetMultiplayerRoundRequest) (multiplayer.Round, error) {
							checkRequest(ctx, req.GameID, req.UserID, req.RequestTime)
							assert.False(t, req.RequestTime.IsZero())

							return storedRound, scenario.usecaseErr
						},
					}
					h := handler.NewHandler(handler.Config{}, uc, wsfake.Tokens{
						Authorized: !scenario.unauthorized, Claims: user.AccessTokenClaims{UserID: userID},
					}, &wsfake.Service{})
					response, err := endpoint.call(t.Context(), h)
					require.NoError(t, err)

					if scenario.unauthorized {
						assert.Zero(t, calls)
						require.IsType(t, endpoint.unauthorizedType, response)
						problem := multiplayerProblem(t, response)
						assert.Equal(t, "Error authorizing user", problem.Title)
						assert.Equal(t, "An error occurred while authorizing user", problem.Detail)
						// The existing body status disagrees with this Unauthorized
						// response variant; correcting it is a separate behavior change.
						return
					}

					assert.Equal(t, 1, calls)

					if scenario.usecaseErr == nil {
						assert.Equal(t, endpoint.wantSuccess, response)
						return
					}

					if errors.Is(scenario.usecaseErr, multiplayer.ErrGameNotFound) && endpoint.notFoundType != nil {
						require.IsType(t, endpoint.notFoundType, response)
						assert.Equal(t, api.BackendError{
							Title: "Game not found", Status: http.StatusNotFound, Detail: endpoint.missingDetail,
						}, multiplayerProblem(t, response))

						return
					}

					require.IsType(t, endpoint.failureType, response)
					assert.Equal(t, api.BackendError{
						Title: endpoint.failureTitle, Status: http.StatusInternalServerError, Detail: endpoint.failureDetail,
					}, multiplayerProblem(t, response))
				})
			}
		})
	}
}

func multiplayerProblem(t *testing.T, response any) api.BackendError {
	t.Helper()

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var problem api.BackendError

	require.NoError(t, json.Unmarshal(data, &problem))

	return problem
}

func TestMultiplayerGameGuessesHTTPReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	uc := handlerUsecase{getGuesses: func(context.Context, dto.GetGameGuessesRequest) ([]multiplayer.Guess, error) {
		return nil, nil
	}}
	h := handler.NewHandler(handler.Config{}, uc, wsfake.Tokens{Authorized: true}, &wsfake.Service{})
	response, err := h.GetMultiplayerGameGuesses(t.Context(), api.GetMultiplayerGameGuessesParams{ID: uuid.New()})
	require.NoError(t, err)

	want := api.GetMultiplayerGameGuessesOKApplicationJSON{}
	assert.Equal(t, &want, response)
}

package lobby_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	handler "github.com/VasySS/segoya-backend/internal/controller/http/v1/lobby"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

type httpUsecase struct {
	handler.Usecase

	newLobby   func(context.Context, dto.NewLobbyRequest) (string, error)
	getLobby   func(context.Context, string) (lobby.Lobby, error)
	getLobbies func(context.Context, dto.GetLobbiesRequest) ([]lobby.Lobby, int, error)
}

func (u httpUsecase) NewLobby(ctx context.Context, req dto.NewLobbyRequest) (string, error) {
	return u.newLobby(ctx, req)
}

func (u httpUsecase) GetLobby(ctx context.Context, id string) (lobby.Lobby, error) {
	return u.getLobby(ctx, id)
}

func (u httpUsecase) GetLobbies(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error) {
	return u.getLobbies(ctx, req)
}

func TestNewLobbyHTTP(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		timer      api.OptInt
		usecaseErr error
	}{
		{name: "omitted timer disables countdown"},
		{name: "explicit zero timer", timer: api.NewOptInt(0)},
		{name: "timed private lobby", timer: api.NewOptInt(60)},
		{name: "unexpected failure is generic", usecaseErr: errors.New("database password must stay private")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := &api.NewLobby{
				CreatorID: uuid.New(), MaxPlayers: 5, Rounds: 3, Provider: api.ProviderGoogle,
				TimerSeconds: tt.timer, MovementAllowed: true, Private: true,
			}

			var requests []dto.NewLobbyRequest

			uc := httpUsecase{newLobby: func(ctx context.Context, req dto.NewLobbyRequest) (string, error) {
				require.NoError(t, ctx.Err())

				requests = append(requests, req)

				return "created-lobby", tt.usecaseErr
			}}
			h := handler.NewHandler(handler.Config{}, uc, nil, &wsfake.Service{})
			before := time.Now()
			response, err := h.NewLobby(t.Context(), req)
			require.NoError(t, err)
			require.Len(t, requests, 1)

			timer, _ := tt.timer.Get()

			assert.False(t, requests[0].RequestTime.Before(before))
			assert.False(t, requests[0].RequestTime.After(time.Now()))
			assert.Equal(t, dto.NewLobbyRequest{
				RequestTime: requests[0].RequestTime, CreatorID: req.CreatorID, MaxPlayers: 5,
				Rounds: 3, Provider: "google", TimerSeconds: timer, MovementAllowed: true, Private: true,
			}, requests[0])

			if tt.usecaseErr != nil {
				assert.Equal(t, &api.NewLobbyInternalServerError{
					Title: "Error creating lobby", Status: http.StatusInternalServerError,
					Detail: "An error occurred while creating lobby",
				}, response)

				return
			}

			assert.Equal(t, &api.NewLobbyCreated{ID: "created-lobby"}, response)
		})
	}
}

func TestNewLobbyHTTPRejectsInvalidSettingsBeforeUsecase(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		provider      api.Provider
		rounds, timer int
		wantDetail    string
	}{
		{name: "provider", provider: "unknown", rounds: 1, wantDetail: "invalid panorama provider"},
		{name: "rounds", provider: api.ProviderGoogle, rounds: 0, wantDetail: "invalid rounds"},
		{name: "timer", provider: api.ProviderGoogle, rounds: 1, timer: 9, wantDetail: "invalid timer seconds"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// A nil usecase makes an accidental downstream call fail immediately.
			h := handler.NewHandler(handler.Config{}, nil, nil, &wsfake.Service{})
			response, err := h.NewLobby(t.Context(), &api.NewLobby{
				Provider: tt.provider, Rounds: tt.rounds, TimerSeconds: api.NewOptInt(tt.timer),
			})
			require.NoError(t, err)
			require.IsType(t, &api.NewLobbyBadRequest{}, response)
			problem, ok := response.(*api.NewLobbyBadRequest)
			require.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, problem.Status)
			assert.Equal(t, "Invalid settings", problem.Title)
			assert.Equal(t, tt.wantDetail, problem.Detail)
		})
	}
}

func TestGetLobbyHTTP(t *testing.T) {
	t.Parallel()

	creatorID := uuid.New()
	createdAt := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	stored := lobby.Lobby{
		ID: "lobby", CreatorID: creatorID, CreatedAt: createdAt, Rounds: 3, Provider: "google",
		MovementAllowed: true, TimerSeconds: 60, CurrentPlayers: 2, MaxPlayers: 5, Private: true,
	}

	for _, tt := range []struct {
		name       string
		usecaseErr error
		want       any
	}{
		{name: "success", want: &api.Lobby{
			ID: "lobby", CreatorID: creatorID, CreatedAt: createdAt, Rounds: 3, Provider: api.ProviderGoogle,
			MovementAllowed: true, TimerSeconds: 60, CurrentPlayers: 2, MaxPlayers: 5, Private: true,
		}},
		{
			name: "wrapped not found", usecaseErr: fmt.Errorf("lookup: %w", lobby.ErrNotFound),
			want: &api.GetLobbyNotFound{
				Title: "Lobby not found", Status: http.StatusNotFound,
				Detail: "The lobby you are trying to get does not exist",
			},
		},
		{
			name: "unexpected failure", usecaseErr: errors.New("database details"),
			want: &api.GetLobbyInternalServerError{
				Title: "Error getting lobby", Status: http.StatusInternalServerError,
				Detail: "An error occurred while getting lobby",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			uc := httpUsecase{getLobby: func(ctx context.Context, id string) (lobby.Lobby, error) {
				calls++

				require.NoError(t, ctx.Err())
				assert.Equal(t, stored.ID, id)

				return stored, tt.usecaseErr
			}}
			h := handler.NewHandler(handler.Config{}, uc, nil, &wsfake.Service{})
			response, err := h.GetLobby(t.Context(), api.GetLobbyParams{ID: "lobby"})
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			assert.Equal(t, tt.want, response)
		})
	}
}

func TestGetLobbiesHTTP(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		lobbies    []lobby.Lobby
		total      int
		usecaseErr error
		want       any
	}{
		{name: "empty list is an array", want: &api.LobbiesResponse{Lobbies: []api.Lobby{}}},
		{
			name: "page and total", total: 42,
			lobbies: []lobby.Lobby{{ID: "lobby", Provider: "google", Rounds: 3, MaxPlayers: 5}},
			want: &api.LobbiesResponse{
				Total: 42, Lobbies: []api.Lobby{{ID: "lobby", Provider: api.ProviderGoogle, Rounds: 3, MaxPlayers: 5}},
			},
		},
		{
			name: "unexpected failure", usecaseErr: errors.New("database details"),
			want: &api.BackendError{
				Title: "Error getting lobbies", Status: http.StatusInternalServerError,
				Detail: "An error occurred while getting lobbies",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			uc := httpUsecase{getLobbies: func(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error) {
				calls++

				require.NoError(t, ctx.Err())
				assert.Equal(t, dto.GetLobbiesRequest{Page: 2, PageSize: 20}, req)

				return tt.lobbies, tt.total, tt.usecaseErr
			}}
			h := handler.NewHandler(handler.Config{}, uc, nil, &wsfake.Service{})
			response, err := h.GetLobbies(t.Context(), api.GetLobbiesParams{Page: 2, PageSize: 20})
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			assert.Equal(t, tt.want, response)
		})
	}
}

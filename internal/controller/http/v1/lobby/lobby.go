package lobby

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

// NewLobby handles the creation of a new lobby.
func (h Handler) NewLobby(
	ctx context.Context,
	req *api.NewLobby,
) (api.NewLobbyRes, error) {
	timerSeconds, _ := req.TimerSeconds.Get()

	lobbyID, err := h.uc.NewLobby(ctx, dto.NewLobbyRequest{
		RequestTime:     time.Now().UTC(),
		CreatorID:       req.CreatorID,
		MaxPlayers:      req.MaxPlayers,
		Rounds:          req.Rounds,
		Provider:        string(req.GetProvider()),
		TimerSeconds:    timerSeconds,
		MovementAllowed: req.MovementAllowed,
	})
	if err != nil {
		slog.Error("error creating lobby", slog.Any("error", err))

		return &api.NewLobbyInternalServerError{
			Title:  "Error creating lobby",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while creating lobby",
		}, nil
	}

	return &api.NewLobbyCreated{ID: lobbyID}, nil
}

// GetLobby retrieves a lobby by its ID.
func (h Handler) GetLobby(
	ctx context.Context,
	params api.GetLobbyParams,
) (api.GetLobbyRes, error) {
	l, err := h.uc.GetLobby(ctx, params.ID)
	if errors.Is(err, lobby.ErrNotFound) {
		return &api.GetLobbyNotFound{
			Title:  "Lobby not found",
			Status: http.StatusNotFound,
			Detail: "The lobby you are trying to get does not exist",
		}, nil
	} else if err != nil {
		slog.Error("error getting lobby", slog.Any("error", err))

		return &api.GetLobbyInternalServerError{
			Title:  "Error getting lobby",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting lobby",
		}, nil
	}

	return dto.LobbyToAPI(l), nil
}

// GetLobbies retrieves all active lobbies.
func (h Handler) GetLobbies(
	ctx context.Context,
	params api.GetLobbiesParams,
) (api.GetLobbiesRes, error) {
	lobbies, total, err := h.uc.GetLobbies(ctx, dto.GetLobbiesRequest{
		Page:     params.Page,
		PageSize: params.PageSize,
	})
	if err != nil {
		slog.Error("error getting lobbies", slog.Any("error", err))

		return &api.Error{
			Title:  "Error getting lobbies",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting lobbies",
		}, nil
	}

	return dto.LobbiesToAPI(lobbies, total), nil
}

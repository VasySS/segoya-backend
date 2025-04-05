package singleplayer

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
)

// NewSingleplayerGame creates new singleplayer game.
func (h Handler) NewSingleplayerGame(
	ctx context.Context,
	req *api.NewSingleplayerGameReq,
) (api.NewSingleplayerGameRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.NewSingleplayerGameUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	timerSeconds, _ := req.TimerSeconds.Get()

	gameID, err := h.uc.NewGame(ctx, dto.NewSingleplayerGameRequest{
		RequestTime:     time.Now().UTC(),
		UserID:          claims.UserID,
		Rounds:          req.Rounds,
		TimerSeconds:    timerSeconds,
		Provider:        string(req.GetProvider()),
		MovementAllowed: req.MovementAllowed,
	})
	if err != nil {
		slog.Error("error creating singleplayer game", slog.Any("error", err))

		return &api.NewSingleplayerGameInternalServerError{
			Title:  "Error creating game",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while creating game",
		}, nil
	}

	return &api.NewSingleplayerGameCreated{
		ID: gameID,
	}, nil
}

// GetSingleplayerGame returns singleplayer game by id.
func (h Handler) GetSingleplayerGame(
	ctx context.Context,
	params api.GetSingleplayerGameParams,
) (api.GetSingleplayerGameRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetSingleplayerGameUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	game, err := h.uc.GetGame(ctx, dto.GetSingleplayerGameRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	})

	switch {
	case errors.Is(err, singleplayer.ErrGameNotFound):
		return &api.GetSingleplayerGameNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game you are trying to get does not exist",
		}, nil
	case errors.Is(err, singleplayer.ErrGameWrongUserID):
		return &api.GetSingleplayerGameForbidden{
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "This game does not belong to you",
		}, nil
	case err != nil:
		slog.Error("error getting singleplayer game", slog.Any("error", err))

		return &api.GetSingleplayerGameInternalServerError{
			Title:  "Error getting game",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting game",
		}, nil
	}

	return dto.SingleplayerGameToAPI(game), nil
}

// EndSingleplayerGame ends singleplayer game.
func (h Handler) EndSingleplayerGame(
	ctx context.Context,
	params api.EndSingleplayerGameParams,
) (api.EndSingleplayerGameRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.EndSingleplayerGameUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	err := h.uc.EndGame(ctx, dto.EndSingleplayerGameRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	})

	switch {
	case errors.Is(err, singleplayer.ErrGameNotFound):
		return &api.EndSingleplayerGameNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil
	case errors.Is(err, singleplayer.ErrGameWrongUserID):
		return &api.EndSingleplayerGameForbidden{
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "This game does not belong to you",
		}, nil
	case errors.Is(err, singleplayer.ErrGameIsStillActive) ||
		errors.Is(err, singleplayer.ErrRoundIsStillActive):
		return &api.EndSingleplayerGameBadRequest{
			Title:  "Game is in progress",
			Status: http.StatusBadRequest,
			Detail: "The game you are trying to end is in progress",
		}, nil
	case err != nil:
		slog.Error("error ending singleplayer game", slog.Any("error", err))

		return &api.EndSingleplayerGameInternalServerError{
			Title:  "Error ending game",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while ending game",
		}, nil
	}

	return &api.EndSingleplayerGameNoContent{}, nil
}

// GetSingleplayerGames returns list of singleplayer games created by user.
func (h Handler) GetSingleplayerGames(
	ctx context.Context,
	params api.GetSingleplayerGamesParams,
) (api.GetSingleplayerGamesRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetSingleplayerGamesUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	games, gamesTotal, err := h.uc.GetGames(ctx, dto.GetSingleplayerGamesRequest{
		UserID:   claims.UserID,
		Page:     params.Page,
		PageSize: params.PageSize,
	})
	if err != nil {
		slog.Error("error getting singleplayer games", slog.Any("error", err))

		return &api.GetSingleplayerGamesInternalServerError{
			Title:  "Error getting games",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting games",
		}, nil
	}

	return dto.SingleplyerGamesToAPI(games, gamesTotal), nil
}

package multiplayer

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
)

// GetMultiplayerGame retrieves a multiplayer game by its ID.
func (h Handler) GetMultiplayerGame(
	ctx context.Context,
	params api.GetMultiplayerGameParams,
) (api.GetMultiplayerGameRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetMultiplayerGameUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	game, err := h.uc.GetGame(ctx, params.ID, claims.UserID)
	if errors.Is(err, multiplayer.ErrGameNotFound) {
		return &api.GetMultiplayerGameNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game you are trying to get does not exist",
		}, nil
	} else if err != nil {
		slog.Error("error getting multiplayer game", slog.Any("error", err))

		return &api.GetMultiplayerGameInternalServerError{
			Title:  "Error getting game",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting game",
		}, nil
	}

	return dto.MultiplayerGameToAPI(game), nil
}

// EndMultiplayerGame ends a multiplayer game and retrieves all user guesses made.
func (h Handler) EndMultiplayerGame(
	ctx context.Context,
	params api.EndMultiplayerGameParams,
) (api.EndMultiplayerGameRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.EndMultiplayerGameUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	guesses, err := h.uc.EndGame(ctx, dto.EndMultiplayerGameRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	})
	if err != nil {
		slog.Error("error getting multiplayer game guesses", slog.Any("error", err))

		return &api.EndMultiplayerGameInternalServerError{
			Title:  "Error getting guesses",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting guesses",
		}, nil
	}

	return dto.MultiplayerGameGuessesToAPI(guesses), nil
}

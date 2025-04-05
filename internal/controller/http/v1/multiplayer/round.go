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

// NewMultiplayerRound creates a new round for a multiplayer game.
func (h Handler) NewMultiplayerRound(
	ctx context.Context,
	params api.NewMultiplayerRoundParams,
) (api.NewMultiplayerRoundRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.NewMultiplayerRoundUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	req := dto.NewMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	}

	round, err := h.uc.NewRound(ctx, req)
	if errors.Is(err, multiplayer.ErrGameNotFound) {
		return &api.NewMultiplayerRoundNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil
	} else if err != nil {
		slog.Error("error creating multiplayer round", slog.Any("error", err))

		return &api.NewMultiplayerRoundInternalServerError{
			Title:  "Error creating round",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while creating round",
		}, nil
	}

	return dto.MultiplayerRoundToAPI(round), nil
}

// GetMultiplayerRound returns current multiplayer round of a game by its id.
func (h Handler) GetMultiplayerRound(
	ctx context.Context,
	params api.GetMultiplayerRoundParams,
) (api.GetMultiplayerRoundRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetMultiplayerRoundUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	req := dto.GetMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	}

	round, err := h.uc.GetRound(ctx, req)
	if errors.Is(err, multiplayer.ErrGameNotFound) {
		return &api.GetMultiplayerRoundNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil
	} else if err != nil {
		slog.Error("error getting multiplayer round", slog.Any("error", err))

		return &api.GetMultiplayerRoundInternalServerError{
			Title:  "Error getting round",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting round",
		}, nil
	}

	return dto.MultiplayerRoundToAPI(round), nil
}

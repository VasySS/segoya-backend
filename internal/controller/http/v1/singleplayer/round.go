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

// NewSingleplayerRound creates new singleplayer round.
func (h Handler) NewSingleplayerRound(
	ctx context.Context,
	params api.NewSingleplayerRoundParams,
) (api.NewSingleplayerRoundRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.NewSingleplayerRoundUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	resp, err := h.uc.NewRound(ctx, dto.NewSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	})

	switch {
	case errors.Is(err, singleplayer.ErrGameNotFound):
		return &api.NewSingleplayerRoundNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil
	case errors.Is(err, singleplayer.ErrGameWrongUserID):
		return &api.NewSingleplayerRoundForbidden{
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "This game does not belong to you",
		}, nil

	case errors.Is(err, singleplayer.ErrRoundMaxAmount):
		return &api.NewSingleplayerRoundBadRequest{
			Title:  "Bad request",
			Status: http.StatusBadRequest,
			Detail: "The maximum amount of rounds has been generated for this game",
		}, nil
	case err != nil:
		slog.Error("error creating singleplayer round", slog.Any("error", err))

		return &api.NewSingleplayerRoundInternalServerError{
			Title:  "Error creating round",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while creating round",
		}, nil
	}

	return dto.SingleplayerRoundToAPI(resp), nil
}

// GetSingleplayerRound returns current singleplayer round by game id.
func (h Handler) GetSingleplayerRound(
	ctx context.Context,
	params api.GetSingleplayerRoundParams,
) (api.GetSingleplayerRoundRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetSingleplayerRoundUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	resp, err := h.uc.GetRound(ctx, dto.GetSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	})

	switch {
	case errors.Is(err, singleplayer.ErrGameNotFound):
		return &api.GetSingleplayerRoundNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil

	case errors.Is(err, singleplayer.ErrGameWrongUserID):
		return &api.GetSingleplayerRoundForbidden{
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "This game does not belong to you",
		}, nil

	case err != nil:
		slog.Error("error getting singleplayer round", slog.Any("error", err))

		return &api.GetSingleplayerRoundInternalServerError{
			Title:  "Error getting round",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting round",
		}, nil
	}

	return dto.SingleplayerRoundToAPI(resp), nil
}

// EndSingleplayerRound ends singleplayer round.
func (h Handler) EndSingleplayerRound(
	ctx context.Context,
	req *api.SingleplayerRoundGuess,
	params api.EndSingleplayerRoundParams,
) (api.EndSingleplayerRoundRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.EndSingleplayerRoundUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	resp, err := h.uc.EndRound(ctx, dto.EndSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
		Guess:       dto.LatLngAPIToEntity(&req.Guess),
	})

	switch {
	case errors.Is(err, singleplayer.ErrRoundAlreadyFinished):
		return &api.EndSingleplayerRoundBadRequest{
			Title:  "Round already finished",
			Status: http.StatusBadRequest,
			Detail: "The round you are trying to end has already finished",
		}, nil

	case errors.Is(err, singleplayer.ErrGameWrongUserID):
		return &api.EndSingleplayerRoundForbidden{
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "This game does not belong to you",
		}, nil

	case errors.Is(err, singleplayer.ErrGameNotFound):
		return &api.EndSingleplayerRoundNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil

	case err != nil:
		slog.Error("error ending singleplayer round", slog.Any("error", err))

		return &api.EndSingleplayerRoundInternalServerError{
			Title:  "Error ending round",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while ending round",
		}, nil
	}

	return dto.SingleplayerRoundResultToAPI(resp), nil
}

// GetSingleplayerGameRounds returns singleplayer rounds of a finished game.
func (h Handler) GetSingleplayerGameRounds(
	ctx context.Context,
	params api.GetSingleplayerGameRoundsParams,
) (api.GetSingleplayerGameRoundsRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetSingleplayerGameRoundsUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	rounds, err := h.uc.GetGameRounds(ctx, dto.GetSingleplayerGameRoundsRequest{
		RequestTime: time.Now().UTC(),
		GameID:      params.ID,
		UserID:      claims.UserID,
	})

	switch {
	case errors.Is(err, singleplayer.ErrGameNotFound):
		return &api.GetSingleplayerGameRoundsNotFound{
			Title:  "Game not found",
			Status: http.StatusNotFound,
			Detail: "The game with the provided ID does not exist",
		}, nil

	case errors.Is(err, singleplayer.ErrGameWrongUserID):
		return &api.GetSingleplayerGameRoundsForbidden{
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "This game does not belong to you",
		}, nil

	case errors.Is(err, singleplayer.ErrGameIsStillActive):
		return &api.GetSingleplayerGameRoundsBadRequest{
			Title:  "Bad request",
			Status: http.StatusBadRequest,
			Detail: "The game is still in progress",
		}, nil

	case err != nil:
		slog.Error("error getting singleplayer game rounds", slog.Any("error", err))

		return &api.GetSingleplayerGameRoundsInternalServerError{
			Title:  "Error getting rounds",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting rounds",
		}, nil
	}

	return dto.SingleplayerRoundsToAPI(rounds), nil
}

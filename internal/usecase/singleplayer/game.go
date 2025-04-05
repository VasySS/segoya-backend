package singleplayer

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
)

// NewGame initializes a new singleplayer game and starts the first round.
func (uc Usecase) NewGame(ctx context.Context, req dto.NewSingleplayerGameRequest) (int, error) {
	ctx, span := uc.tracer.Start(ctx, "NewGame")
	defer span.End()

	var response int

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		id, err := uc.repo.NewSingleplayerGame(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to create singleplayer game: %w", err)
		}

		_, err = uc.NewRound(ctx, dto.NewSingleplayerRoundRequest{
			RequestTime: req.RequestTime,
			GameID:      id,
			UserID:      req.UserID,
		})
		if err != nil {
			return fmt.Errorf("failed to create round: %w", err)
		}

		response = id

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to create game: %w", err)
	}

	return response, nil
}

// GetGame returns a singleplayer game by ID.
func (uc Usecase) GetGame(ctx context.Context, req dto.GetSingleplayerGameRequest) (singleplayer.Game, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGame")
	defer span.End()

	game, err := uc.repo.GetSingleplayerGame(ctx, req.GameID)
	if err != nil {
		span.RecordError(err)
		return singleplayer.Game{}, fmt.Errorf("failed to get singleplayer game: %w", err)
	}

	if game.UserID != req.UserID {
		return singleplayer.Game{}, singleplayer.ErrGameWrongUserID
	}

	return game, nil
}

// EndGame ends a singleplayer game (if it's not finished already).
func (uc Usecase) EndGame(ctx context.Context, req dto.EndSingleplayerGameRequest) error {
	ctx, span := uc.tracer.Start(ctx, "EndGame")
	defer span.End()

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		game, err := uc.repo.GetSingleplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game from db: %w", err)
		}

		if game.UserID != req.UserID {
			return singleplayer.ErrGameWrongUserID
		}

		if game.RoundCurrent != game.Rounds {
			return singleplayer.ErrGameIsStillActive
		}

		round, err := uc.repo.GetSingleplayerRound(ctx, req.GameID, game.RoundCurrent)
		if err != nil {
			return fmt.Errorf("failed to get current round from db: %w", err)
		}

		if !round.Finished {
			return singleplayer.ErrRoundIsStillActive
		}

		if err := uc.repo.EndSingleplayerGame(ctx, dto.EndSingleplayerGameRequestDB{
			RequestTime: req.RequestTime,
			GameID:      req.GameID,
		}); err != nil {
			return fmt.Errorf("failed to end game in repo: %w", err)
		}

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to end game: %w", err)
	}

	return nil
}

// GetGames returns a list of singleplayer games created by user.
func (uc Usecase) GetGames(
	ctx context.Context,
	req dto.GetSingleplayerGamesRequest,
) ([]singleplayer.Game, int, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGames")
	defer span.End()

	games, gamesTotal, err := uc.repo.GetSingleplayerGames(ctx, req)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to get singleplayer games: %w", err)
	}

	return games, gamesTotal, nil
}

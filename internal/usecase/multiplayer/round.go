package multiplayer

import (
	"context"
	"errors"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
)

// NewRound creates a new multiplayer game round or returns an existing one if it's not finished.
func (uc Usecase) NewRound(
	ctx context.Context,
	req dto.NewMultiplayerRoundRequest,
) (multiplayer.Round, error) {
	ctx, span := uc.tracer.Start(ctx, "NewRound")
	defer span.End()

	var response multiplayer.Round

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockMultiplayerGame(ctx, req.GameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		if err := uc.isUserInGame(ctx, req.UserID, req.GameID); err != nil {
			return err
		}

		game, err := uc.repo.GetMultiplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get multiplayer game: %w", err)
		}

		if existingRound, err := uc.repo.GetMultiplayerRound(ctx, game.ID, game.RoundCurrent); err == nil {
			newRoundDelayEnd := existingRound.EndedAt.Add(uc.cfg.RoundEndDelay)

			if !existingRound.Finished || req.RequestTime.Before(newRoundDelayEnd) {
				response = existingRound
				return nil
			}
		} else if !errors.Is(err, multiplayer.ErrRoundNotFound) {
			return fmt.Errorf("failed to get current round: %w", err)
		}

		if game.RoundCurrent != 0 && game.RoundCurrent == game.Rounds {
			return multiplayer.ErrRoundMaxAmount
		}

		pano, err := uc.pano.NewStreetview(ctx, game.Provider)
		if err != nil {
			return fmt.Errorf("failed to create panorama: %w", err)
		}

		dbReq := dto.NewMultiplayerRoundRequestDB{
			GameID:     game.ID,
			LocationID: pano.ID,
			RoundNum:   game.RoundCurrent + 1,
			CreatedAt:  req.RequestTime,
			StartedAt:  req.RequestTime.Add(uc.cfg.RoundStartDelay),
		}

		round, err := uc.repo.NewMultiplayerRound(ctx, dbReq)
		if err != nil {
			return fmt.Errorf("failed to create round: %w", err)
		}

		response = round

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return multiplayer.Round{}, fmt.Errorf("failed to create round: %w", err)
	}

	return response, nil
}

// GetRound returns current multiplayer game round by game ID.
func (uc Usecase) GetRound(
	ctx context.Context,
	req dto.GetMultiplayerRoundRequest,
) (multiplayer.Round, error) {
	ctx, span := uc.tracer.Start(ctx, "GetRound")
	defer span.End()

	var response multiplayer.Round

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockMultiplayerGame(ctx, req.GameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		if err := uc.isUserInGame(ctx, req.UserID, req.GameID); err != nil {
			return err
		}

		game, err := uc.repo.GetMultiplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get multiplayer game: %w", err)
		}

		round, err := uc.repo.GetMultiplayerRound(ctx, game.ID, game.RoundCurrent)
		if err != nil {
			return fmt.Errorf("failed to get multiplayer round: %w", err)
		}

		response = round

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return multiplayer.Round{}, fmt.Errorf("failed to get round: %w", err)
	}

	return response, nil
}

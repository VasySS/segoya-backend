package multiplayer

import (
	"context"
	"errors"
	"fmt"
	"time"

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

		round, err := uc.getOrGenerateRound(ctx, game, req.RequestTime)
		if err != nil {
			return err
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

func (uc Usecase) getOrGenerateRound(
	ctx context.Context,
	game multiplayer.Game,
	requestTime time.Time,
) (multiplayer.Round, error) {
	if existingRound, err := uc.repo.GetMultiplayerRound(ctx, game.ID, game.RoundCurrent); err == nil {
		if existingRound.CanReuse(requestTime, uc.cfg.RoundEndDelay) {
			return existingRound, nil
		}
	} else if !errors.Is(err, multiplayer.ErrRoundNotFound) {
		return multiplayer.Round{}, fmt.Errorf("failed to get current round: %w", err)
	}

	if game.MaxRoundsReached() {
		return multiplayer.Round{}, multiplayer.ErrRoundMaxAmount
	}

	pano, err := uc.pano.NewStreetview(ctx, game.Provider)
	if err != nil {
		return multiplayer.Round{}, fmt.Errorf("failed to create panorama: %w", err)
	}

	creation := game.PrepareNextRound(pano, requestTime, uc.cfg.RoundStartDelay)
	dbReq := dto.NewMultiplayerRoundRequestDB{
		GameID:     creation.GameID,
		LocationID: creation.LocationID,
		RoundNum:   creation.RoundNum,
		CreatedAt:  creation.CreatedAt,
		StartedAt:  creation.StartedAt,
	}

	round, err := uc.repo.NewMultiplayerRound(ctx, dbReq)
	if err != nil {
		return multiplayer.Round{}, fmt.Errorf("failed to create round: %w", err)
	}

	return round, nil
}

// GetRound returns current multiplayer game's round by game id.
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

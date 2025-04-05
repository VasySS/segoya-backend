package singleplayer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
)

// NewRound creates a new singleplayer game round or returns an existing one if it's not finished.
func (uc Usecase) NewRound(
	ctx context.Context,
	req dto.NewSingleplayerRoundRequest,
) (singleplayer.Round, error) {
	ctx, span := uc.tracer.Start(ctx, "NewRound")
	defer span.End()

	var response singleplayer.Round

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockSingleplayerGame(ctx, req.GameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		game, err := uc.repo.GetSingleplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get singleplayer game: %w", err)
		}

		if game.UserID != req.UserID {
			return singleplayer.ErrGameWrongUserID
		}

		if game.RoundCurrent != 0 && game.RoundCurrent == game.Rounds {
			return singleplayer.ErrRoundMaxAmount
		}

		if existingRound, err := uc.repo.GetSingleplayerRound(ctx, game.ID, game.RoundCurrent); err == nil {
			if !existingRound.Finished {
				response = existingRound
				return nil
			}
		} else if !errors.Is(err, singleplayer.ErrRoundNotFound) {
			return fmt.Errorf("failed to get current round: %w", err)
		}

		pano, err := uc.pano.NewStreetview(ctx, game.Provider)
		if err != nil {
			return fmt.Errorf("failed to create panorama: %w", err)
		}

		dbReq := dto.NewSingleplayerRoundDBRequest{
			GameID:     game.ID,
			LocationID: pano.ID,
			RoundNum:   game.RoundCurrent + 1,
			CreatedAt:  req.RequestTime,
			StartedAt:  req.RequestTime.Add(uc.cfg.RoundStartDelay),
		}

		round, err := uc.repo.NewSingleplayerRound(ctx, dbReq)
		if err != nil {
			return fmt.Errorf("error creating singleplayer round: %w", err)
		}

		response = round

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return singleplayer.Round{}, fmt.Errorf("failed to create new round: %w", err)
	}

	return response, nil
}

// GetRound returns current singleplayer game round by game ID.
func (uc Usecase) GetRound(
	ctx context.Context,
	req dto.GetSingleplayerRoundRequest,
) (singleplayer.Round, error) {
	ctx, span := uc.tracer.Start(ctx, "GetRound")
	defer span.End()

	var response singleplayer.Round

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockSingleplayerGame(ctx, req.GameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		game, err := uc.repo.GetSingleplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get singleplayer game: %w", err)
		}

		if game.UserID != req.UserID {
			return singleplayer.ErrGameWrongUserID
		}

		round, err := uc.repo.GetSingleplayerRound(ctx, game.ID, game.RoundCurrent)
		if err != nil {
			return fmt.Errorf("failed to get round: %w", err)
		}

		response = round

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return singleplayer.Round{}, fmt.Errorf("failed to get round: %w", err)
	}

	return response, nil
}

// EndRound ends a singleplayer game round (if it's not finished already) and
// returns all guesses made during it.
func (uc Usecase) EndRound(
	ctx context.Context,
	req dto.EndSingleplayerRoundRequest,
) (dto.EndCurrentRoundResponse, error) {
	ctx, span := uc.tracer.Start(ctx, "EndRound")
	defer span.End()

	var response dto.EndCurrentRoundResponse

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		game, err := uc.repo.GetSingleplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get singleplayer game: %w", err)
		}

		if game.UserID != req.UserID {
			return singleplayer.ErrGameWrongUserID
		}

		round, err := uc.repo.GetSingleplayerRound(ctx, game.ID, game.RoundCurrent)
		if err != nil {
			return fmt.Errorf("failed to get current round from db: %w", err)
		}

		if round.Finished {
			return singleplayer.ErrRoundAlreadyFinished
		}

		score, distance := uc.pano.CalculateScoreAndDistance(
			game.Provider, round.Lat, round.Lng, req.Guess.Lat, req.Guess.Lng,
		)

		roundTimerEnd := round.StartedAt.Add(time.Second * time.Duration(game.TimerSeconds))
		if game.TimerSeconds != 0 && req.RequestTime.After(roundTimerEnd) {
			// if timer is enabled and round timer has ended, reset score
			score = 0
		}

		dbReq := dto.NewSingleplayerRoundGuessRequest{
			RequestTime: req.RequestTime,
			RoundID:     round.ID,
			GameID:      req.GameID,
			Guess:       req.Guess,
			Score:       score,
			Distance:    distance,
		}

		if err := uc.repo.NewSingleplayerRoundGuess(ctx, dbReq); err != nil {
			return fmt.Errorf("failed to set user guess in db: %w", err)
		}

		response = dto.EndCurrentRoundResponse{
			Score:    score,
			Distance: distance,
		}

		return nil
	})
	if err != nil {
		return dto.EndCurrentRoundResponse{}, fmt.Errorf("failed to end round: %w", err)
	}

	return response, nil
}

// GetGameRounds returns all rounds of a singleplayer game with guess results.
func (uc Usecase) GetGameRounds(
	ctx context.Context,
	req dto.GetSingleplayerGameRoundsRequest,
) ([]singleplayer.Guess, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGameRounds")
	defer span.End()

	var response []singleplayer.Guess

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		game, err := uc.repo.GetSingleplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game from db: %w", err)
		}

		if game.UserID != req.UserID {
			return singleplayer.ErrGameWrongUserID
		}

		if !game.Finished {
			return singleplayer.ErrGameIsStillActive
		}

		rounds, err := uc.repo.GetSingleplayerGameGuesses(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get rounds from db: %w", err)
		}

		response = rounds

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get game rounds: %w", err)
	}

	return response, nil
}

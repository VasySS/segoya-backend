package multiplayer

import (
	"context"
	"fmt"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
)

// NewRoundGuess saves a user's guess for current round (called from websocket).
func (uc Usecase) NewRoundGuess(ctx context.Context, req dto.NewMultiplayerRoundGuessRequest) error {
	ctx, span := uc.tracer.Start(ctx, "NewRoundGuess")
	defer span.End()

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		multiplayerGame, err := uc.repo.GetMultiplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game: %w", err)
		}

		if err := uc.isUserInGame(ctx, req.UserID, req.GameID); err != nil {
			return err
		}

		round, err := uc.repo.GetMultiplayerRound(ctx, multiplayerGame.ID, multiplayerGame.RoundCurrent)
		if err != nil {
			return fmt.Errorf("failed to get current round: %w", err)
		}

		if round.Finished {
			return multiplayer.ErrRoundAlreadyFinished
		}

		score, distance := uc.pano.CalculateScoreAndDistance(
			multiplayerGame.Provider,
			round.Lat,
			round.Lng,
			req.Guess.Lat,
			req.Guess.Lng,
		)

		err = uc.repo.NewMultiplayerRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequestDB{
			RequestTime: req.RequestTime,
			RoundID:     round.ID,
			UserID:      req.UserID,
			Score:       score,
			Distance:    distance,
			Lat:         req.Guess.Lat,
			Lng:         req.Guess.Lng,
		})
		if err != nil {
			return fmt.Errorf("failed to set user guess: %w", err)
		}

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to set user guess: %w", err)
	}

	return nil
}

// EndRound ends a multiplayer game round (if it's not finished already) and
// returns all guesses made during it (called from websocket).
func (uc Usecase) EndRound(ctx context.Context, req dto.EndMultiplayerRoundRequest) ([]multiplayer.Guess, error) {
	ctx, span := uc.tracer.Start(ctx, "EndRound")
	defer span.End()

	var response []multiplayer.Guess

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockMultiplayerGame(ctx, req.GameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		if err := uc.isUserInGame(ctx, req.UserID, req.GameID); err != nil {
			return err
		}

		g, err := uc.repo.GetMultiplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game: %w", err)
		}

		r, err := uc.repo.GetMultiplayerRound(ctx, g.ID, g.RoundCurrent)
		if err != nil {
			return fmt.Errorf("failed to get current round: %w", err)
		}

		gs, err := uc.repo.GetMultiplayerRoundGuesses(ctx, r.ID)
		if err != nil {
			return fmt.Errorf("failed to get rounds: %w", err)
		}

		// if round is already finished, just return guesses
		if r.Finished {
			response = gs
			return nil
		}

		timerEndTime := r.StartedAt.Add(time.Second * time.Duration(g.TimerSeconds))
		if g.TimerSeconds != 0 && req.RequestTime.Before(timerEndTime) && r.GuessesCount != g.Players {
			return multiplayer.ErrRoundIsStillActive
		}

		req := dto.EndMultiplayerRoundRequestDB{
			RequestTime: req.RequestTime,
			RoundID:     r.ID,
		}
		if err := uc.repo.EndMultiplayerRound(ctx, req); err != nil {
			return fmt.Errorf("failed to update round end in repo: %w", err)
		}

		response = gs

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to end round: %w", err)
	}

	return response, nil
}

package singleplayer

import (
	"context"
	"fmt"
	"time"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
)

func (uc Usecase) scoreGuess(
	ctx context.Context,
	g singleplayer.Game,
	round singleplayer.Round,
	requestTime time.Time,
	guess *game.LatLng,
) (int, int, error) {
	score, distance, err := round.EvaluateGuess(g, requestTime, guess)
	if err != nil {
		return score, distance, err //nolint:wrapcheck // Keep domain validation errors intact.
	}

	if g.Finished {
		return 0, 0, game.ErrInvalidState
	}

	if g.Scoring != nil {
		return score, distance, nil
	}

	if uc.areas == nil {
		return 0, 0, errAreaServiceNotConfigured
	}

	scoring, err := uc.areas.ResolveScoring(ctx, g.Provider, g.PlayArea)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve game scoring: %w", err)
	}

	if err := uc.repo.InitializeSingleplayerScoring(ctx, g.ID, scoring); err != nil {
		return 0, 0, fmt.Errorf("freeze game scoring: %w", err)
	}

	g.Scoring = &scoring

	return round.EvaluateGuess(g, requestTime, guess) //nolint:wrapcheck // Keep domain validation errors intact.
}

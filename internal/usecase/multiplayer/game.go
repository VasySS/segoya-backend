package multiplayer

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// NewGame initializes a new multiplayer game and starts the first round.
func (uc Usecase) NewGame(ctx context.Context, req dto.NewMultiplayerGameRequest) (int, error) {
	ctx, span := uc.tracer.Start(ctx, "NewGame")
	defer span.End()

	var response int

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		gID, err := uc.repo.NewMultiplayerGame(ctx, req)
		if err != nil {
			return fmt.Errorf("error creating multiplayer game: %w", err)
		}

		_, err = uc.NewRound(ctx, dto.NewMultiplayerRoundRequest{
			RequestTime: req.RequestTime,
			GameID:      gID,
			UserID:      req.CreatorID,
		})
		if err != nil {
			return fmt.Errorf("failed to create round: %w", err)
		}

		response = gID

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("failed to create game: %w", err)
	}

	return response, nil
}

// GetGame returns a multiplayer game by ID.
func (uc Usecase) GetGame(ctx context.Context, gameID, userID int) (multiplayer.Game, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGame")
	defer span.End()

	var response multiplayer.Game

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockMultiplayerGame(ctx, gameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		if err := uc.isUserInGame(ctx, userID, gameID); err != nil {
			return err
		}

		game, err := uc.repo.GetMultiplayerGame(ctx, gameID)
		if err != nil {
			return fmt.Errorf("failed to get multiplayer game: %w", err)
		}

		response = game

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return multiplayer.Game{}, fmt.Errorf("failed to get game: %w", err)
	}

	return response, nil
}

// EndGame ends a multiplayer game (if it's not finished already) and
// returns all guesses made during it.
func (uc Usecase) EndGame(ctx context.Context, req dto.EndMultiplayerGameRequest) ([]multiplayer.Guess, error) {
	ctx, span := uc.tracer.Start(ctx, "EndGame")
	defer span.End()

	var response []multiplayer.Guess

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockMultiplayerGame(ctx, req.GameID); err != nil {
			return fmt.Errorf("failed to lock game: %w", err)
		}

		if err := uc.isUserInGame(ctx, req.UserID, req.GameID); err != nil {
			return err
		}

		game, err := uc.repo.GetMultiplayerGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game from repo: %w", err)
		}

		if game.RoundCurrent < game.Rounds {
			return multiplayer.ErrGameIsStillActive
		}

		gs, err := uc.GetGameGuesses(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game guesses: %w", err)
		}

		if game.Finished {
			response = gs
			return nil
		}

		if err := uc.repo.EndMultiplayerGame(ctx, dto.EndMultiplayerGameRequestDB{
			RequestTime: req.RequestTime,
			GameID:      req.GameID,
		}); err != nil {
			return fmt.Errorf("failed to update game end in repo: %w", err)
		}

		response = gs

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to end game: %w", err)
	}

	return response, nil
}

// GetGameGuesses returns all guesses made during a game.
func (uc Usecase) GetGameGuesses(ctx context.Context, gameID int) ([]multiplayer.Guess, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGameGuesses")
	defer span.End()

	guesses, err := uc.repo.GetMultiplayerGameGuesses(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get multiplayer game guesses: %w", err)
	}

	return guesses, nil
}

// GetGameUsers returns all users info in a game (including those, who left).
func (uc Usecase) GetGameUsers(ctx context.Context, gameID int) ([]user.MultiplayerUser, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGameUsers")
	defer span.End()

	users, err := uc.repo.GetMultiplayerGameUsers(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get multiplayer game users: %w", err)
	}

	return users, nil
}

// GetGameUser returns a user info in a game.
func (uc Usecase) GetGameUser(ctx context.Context, userID, gameID int) (user.MultiplayerUser, error) {
	ctx, span := uc.tracer.Start(ctx, "GetGameUser")
	defer span.End()

	userDB, err := uc.repo.GetMultiplayerGameUser(ctx, userID, gameID)
	if err != nil {
		span.RecordError(err)
		return user.MultiplayerUser{}, fmt.Errorf("failed to get user: %w", err)
	}

	return userDB, nil
}

// isUserInGame validates a user's participation in a game.
func (uc Usecase) isUserInGame(ctx context.Context, userID, gameID int) error {
	users, err := uc.repo.GetMultiplayerGameUsers(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get multiplayer game users: %w", err)
	}

	for _, u := range users {
		if u.ID == userID {
			return nil
		}
	}

	return multiplayer.ErrGameWrongUserID
}

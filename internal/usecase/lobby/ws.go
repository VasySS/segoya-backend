package lobby

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// ConnectLobbyUser handles a user joining a lobby (called from the websocket).
func (uc Usecase) ConnectLobbyUser(
	ctx context.Context,
	lobbyID string,
	userID int,
) (user.PublicProfile, error) {
	ctx, span := uc.tracer.Start(ctx, "ConnectLobbyUser")
	defer span.End()

	lobbyRepo, err := uc.lobbyRepo.GetLobby(ctx, lobbyID)
	if err != nil {
		return user.PublicProfile{}, fmt.Errorf("error getting lobby: %w", err)
	}

	if lobbyRepo.CurrentPlayers >= lobbyRepo.MaxPlayers {
		return user.PublicProfile{}, lobby.ErrLobbyIsFull
	}

	if err := uc.lobbyRepo.IncrementLobbyPlayers(ctx, lobbyID); err != nil {
		return user.PublicProfile{}, fmt.Errorf("error incrementing current players: %w", err)
	}

	if err := uc.lobbyRepo.DeleteLobbyExpiration(ctx, lobbyID); err != nil {
		return user.PublicProfile{}, fmt.Errorf("error deleting lobby expiration: %w", err)
	}

	userRepo, err := uc.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return user.PublicProfile{}, fmt.Errorf("error getting user profile: %w", err)
	}

	return userRepo.ToPublicProfile(), nil
}

// DisconnectLobbyUser handles a user leaving a lobby (called from the websocket).
func (uc Usecase) DisconnectLobbyUser(
	ctx context.Context,
	lobbyID string,
	_ int,
) error {
	ctx, span := uc.tracer.Start(ctx, "DisconnectLobbyUser")
	defer span.End()

	lobby, err := uc.lobbyRepo.GetLobby(ctx, lobbyID)
	if err != nil {
		return fmt.Errorf("error getting lobby from db: %w", err)
	}

	// delete lobby if it is empty for some time
	if lobby.CurrentPlayers == 1 {
		if err := uc.lobbyRepo.AddLobbyExpiration(ctx, lobbyID, uc.conf.LobbyExpiration); err != nil {
			return fmt.Errorf("error deleting lobby: %w", err)
		}
	}

	if err := uc.lobbyRepo.DecrementLobbyPlayers(ctx, lobbyID); err != nil {
		return fmt.Errorf("error decrementing current players: %w", err)
	}

	return nil
}

// StartLobbyGame initiates a multiplayer game from a lobby (called from the websocket).
func (uc Usecase) StartLobbyGame(
	ctx context.Context,
	req dto.StartLobbyGameRequest,
) (int, error) {
	ctx, span := uc.tracer.Start(ctx, "StartLobbyGame")
	defer span.End()

	lobbyRepo, err := uc.lobbyRepo.GetLobby(ctx, req.LobbyID)
	if err != nil {
		return 0, fmt.Errorf("failed to get lobby from db: %w", err)
	}

	if lobbyRepo.CreatorID != req.Creator.ID {
		return 0, lobby.ErrOnlyCreatorCanStart
	}

	gameID, err := uc.mult.NewGame(ctx, dto.NewMultiplayerGameRequest{
		RequestTime:      req.RequestTime,
		CreatorID:        req.Creator.ID,
		ConnectedPlayers: req.ConnectedPlayers,
		Rounds:           lobbyRepo.Rounds,
		TimerSeconds:     lobbyRepo.TimerSeconds,
		MovementAllowed:  lobbyRepo.MovementAllowed,
		Provider:         lobbyRepo.Provider,
	})
	if err != nil {
		return 0, fmt.Errorf("error starting game: %w", err)
	}

	if err := uc.DeleteLobby(ctx, req.LobbyID); err != nil {
		return 0, fmt.Errorf("error deleting lobby: %w", err)
	}

	return gameID, nil
}

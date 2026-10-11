package lobby

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// ConnectLobbyUser handles a user joining a lobby (called from the websocket) and updates the lobby's state.
func (uc Usecase) ConnectLobbyUser(
	ctx context.Context,
	lobbyID string,
	userID uuid.UUID,
) (user.PublicProfile, error) {
	ctx, span := uc.tracer.Start(ctx, "ConnectLobbyUser")
	defer span.End()

	lobbyRepo, err := uc.lobbyRepo.GetLobby(ctx, lobbyID)
	if err != nil {
		span.RecordError(err)
		return user.PublicProfile{}, fmt.Errorf("error getting lobby: %w", err)
	}

	if lobbyRepo.IsFull() {
		return user.PublicProfile{}, lobby.ErrLobbyIsFull
	}

	if err := uc.lobbyRepo.IncrementLobbyPlayers(ctx, lobbyID); err != nil {
		span.RecordError(err)
		return user.PublicProfile{}, fmt.Errorf("error incrementing current players: %w", err)
	}

	if err := uc.lobbyRepo.DeleteLobbyExpiration(ctx, lobbyID); err != nil {
		span.RecordError(err)
		return user.PublicProfile{}, fmt.Errorf("error deleting lobby expiration: %w", err)
	}

	userRepo, err := uc.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return user.PublicProfile{}, fmt.Errorf("error getting user profile: %w", err)
	}

	return userRepo.ToPublicProfile(), nil
}

// DisconnectLobbyUser handles a user leaving a lobby (called from the websocket) and updates the lobby's state.
func (uc Usecase) DisconnectLobbyUser(
	ctx context.Context,
	lobbyID string,
	_ uuid.UUID,
) error {
	ctx, span := uc.tracer.Start(ctx, "DisconnectLobbyUser")
	defer span.End()

	lobbyRepo, err := uc.lobbyRepo.GetLobby(ctx, lobbyID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("error getting lobby from db: %w", err)
	}

	// set the lobby expiration timer if the leaving user is the last one
	if lobbyRepo.IsLastPlayer() {
		if err := uc.lobbyRepo.AddLobbyExpiration(ctx, lobbyID, uc.conf.LobbyExpiration); err != nil {
			span.RecordError(err)
			return fmt.Errorf("error deleting lobby: %w", err)
		}
	}

	if err := uc.lobbyRepo.DecrementLobbyPlayers(ctx, lobbyID); err != nil {
		span.RecordError(err)
		return fmt.Errorf("error decrementing current players: %w", err)
	}

	return nil
}

// StartLobbyGame starts a new multiplayer game from a lobby (called from the websocket by the lobby creator).
func (uc Usecase) StartLobbyGame(
	ctx context.Context,
	req dto.StartLobbyGameRequest,
) (uuid.UUID, error) {
	ctx, span := uc.tracer.Start(ctx, "StartLobbyGame")
	defer span.End()

	lobbyRepo, err := uc.lobbyRepo.GetLobby(ctx, req.LobbyID)
	if err != nil {
		span.RecordError(err)
		return uuid.UUID{}, fmt.Errorf("failed to get lobby from db: %w", err)
	}

	if err := lobbyRepo.ValidateStart(req.Creator.ID); err != nil {
		return uuid.UUID{}, err //nolint:wrapcheck // Preserve the existing unwrapped permission error.
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
		span.RecordError(err)
		return uuid.UUID{}, fmt.Errorf("error starting game: %w", err)
	}

	if err := uc.lobbyRepo.DeleteLobby(ctx, req.LobbyID); err != nil {
		span.RecordError(err)
		return uuid.UUID{}, fmt.Errorf("failed to delete lobby: %w", err)
	}

	return gameID, nil
}

// UpdateLobbySettings updates the settings of a lobby (called from the websocket by the lobby creator).
func (uc Usecase) UpdateLobbySettings(
	ctx context.Context,
	req dto.UpdateLobbySettingsRequest,
) error {
	ctx, span := uc.tracer.Start(ctx, "UpdateLobbySettings")
	defer span.End()

	lobbyInfo, err := uc.lobbyRepo.GetLobby(ctx, req.LobbyID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("error getting lobby from db: %w", err)
	}

	lobbyInfo, err = lobbyInfo.WithSettings(req.Creator.ID, req.Settings)
	if err != nil {
		return err //nolint:wrapcheck // Preserve the existing unwrapped permission error.
	}

	if err := uc.lobbyRepo.UpdateLobby(ctx, lobbyInfo); err != nil {
		span.RecordError(err)
		return fmt.Errorf("error updating lobby settings: %w", err)
	}

	return nil
}

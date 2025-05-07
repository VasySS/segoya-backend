package lobby

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

// NewLobby creates a new multiplayer lobby and returns its ID.
func (uc Usecase) NewLobby(ctx context.Context, req dto.NewLobbyRequest) (string, error) {
	ctx, span := uc.tracer.Start(ctx, "NewLobby")
	defer span.End()

	id := uc.rnd.NewRandomHexString(uc.conf.LobbyIDLength)

	dbReq := dto.NewLobbyRequestDB{
		ID:              id,
		RequestTime:     req.RequestTime,
		MaxPlayers:      req.MaxPlayers,
		CreatorID:       req.CreatorID,
		Rounds:          req.Rounds,
		Provider:        req.Provider,
		TimerSeconds:    req.TimerSeconds,
		MovementAllowed: req.MovementAllowed,
	}

	if req.Private {
		if err := uc.lobbyRepo.NewPrivateLobby(ctx, dbReq); err != nil {
			span.RecordError(err)
			return "", fmt.Errorf("failed to create private lobby: %w", err)
		}
	} else {
		if err := uc.lobbyRepo.NewLobby(ctx, dbReq); err != nil {
			span.RecordError(err)
			return "", fmt.Errorf("failed to create lobby: %w", err)
		}
	}

	if err := uc.lobbyRepo.AddLobbyExpiration(ctx, id, uc.conf.LobbyExpiration); err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to add lobby expiration: %w", err)
	}

	return id, nil
}

// GetLobby retrieves a lobby's information by its ID.
func (uc Usecase) GetLobby(ctx context.Context, id string) (lobby.Lobby, error) {
	ctx, span := uc.tracer.Start(ctx, "GetLobby")
	defer span.End()

	l, err := uc.lobbyRepo.GetLobby(ctx, id)
	if err != nil {
		span.RecordError(err)
		return lobby.Lobby{}, fmt.Errorf("failed to get lobby from db: %w", err)
	}

	return l, nil
}

// GetLobbies retrieves all (public) lobbies available.
func (uc Usecase) GetLobbies(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error) {
	ctx, span := uc.tracer.Start(ctx, "GetLobbies")
	defer span.End()

	l, total, err := uc.lobbyRepo.GetLobbies(ctx, req)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to get lobbies from db: %w", err)
	}

	return l, total, nil
}

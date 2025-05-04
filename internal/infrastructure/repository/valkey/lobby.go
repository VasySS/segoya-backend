package valkey

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

const (
	lobbyPrefix               = "lobby:"
	lobbiesPrefix             = "lobbies:sorted"
	lobbyIDField              = "id"
	lobbyCreatorIDField       = "creatorID"
	lobbyCreatedAtField       = "createdAt"
	lobbyRoundsField          = "rounds"
	lobbyProviderField        = "provider"
	lobbyTimerSecondsField    = "timerSeconds"
	lobbyMovementAllowedField = "movementAllowed"
	lobbyMaxPlayersField      = "maxPlayers"
	lobbyCurrentPlayersField  = "currentPlayers"
)

// NewLobby creates new lobby in the database.
func (r *Repository) NewLobby(ctx context.Context, req dto.NewLobbyRequestDB) error {
	ctx, span := r.tracer.Start(ctx, "NewLobby")
	defer span.End()

	key := lobbyPrefix + req.ID
	fields := map[string]string{
		lobbyIDField:              req.ID,
		lobbyCreatorIDField:       strconv.Itoa(req.CreatorID),
		lobbyCreatedAtField:       req.RequestTime.Format(time.RFC3339),
		lobbyRoundsField:          strconv.Itoa(req.Rounds),
		lobbyProviderField:        req.Provider,
		lobbyTimerSecondsField:    strconv.Itoa(req.TimerSeconds),
		lobbyMovementAllowedField: strconv.FormatBool(req.MovementAllowed),
		lobbyMaxPlayersField:      strconv.Itoa(req.MaxPlayers),
		lobbyCurrentPlayersField:  "0",
	}

	cmd := r.valkey.B().Hset().Key(key).FieldValue()
	for field, value := range fields {
		cmd = cmd.FieldValue(field, value)
	}

	if err := r.valkey.Do(ctx, cmd.Build()).Error(); err != nil {
		return fmt.Errorf("failed to create lobby: %w", err)
	}

	addSorted := r.valkey.B().Zadd().Key(lobbiesPrefix).ScoreMember().
		ScoreMember(float64(req.RequestTime.Unix()), req.ID)

	if err := r.valkey.Do(ctx, addSorted.Build()).Error(); err != nil {
		return fmt.Errorf("failed to add lobby to sorted set: %w", err)
	}

	return nil
}

// GetLobby gets lobby from the database.
func (r *Repository) GetLobby(ctx context.Context, id string) (lobby.Lobby, error) {
	ctx, span := r.tracer.Start(ctx, "Lobby")
	defer span.End()

	key := lobbyPrefix + id
	cmd := r.valkey.B().Hgetall().Key(key).Build()

	resp, err := r.valkey.Do(ctx, cmd).AsStrMap()
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("failed to get lobby: %w", err)
	}

	if len(resp) == 0 {
		return lobby.Lobby{}, lobby.ErrNotFound
	}

	return parseLobbyData(id, resp)
}

// UpdateLobby updates lobby in the database.
func (r *Repository) UpdateLobby(ctx context.Context, lobbyInfo lobby.Lobby) error {
	ctx, span := r.tracer.Start(ctx, "UpdateLobby")
	defer span.End()

	key := lobbyPrefix + lobbyInfo.ID
	fields := map[string]string{
		lobbyRoundsField:          strconv.Itoa(lobbyInfo.Rounds),
		lobbyProviderField:        lobbyInfo.Provider,
		lobbyTimerSecondsField:    strconv.Itoa(lobbyInfo.TimerSeconds),
		lobbyMovementAllowedField: strconv.FormatBool(lobbyInfo.MovementAllowed),
	}

	cmd := r.valkey.B().Hset().Key(key).FieldValue()
	for field, value := range fields {
		cmd = cmd.FieldValue(field, value)
	}

	if err := r.valkey.Do(ctx, cmd.Build()).Error(); err != nil {
		return fmt.Errorf("failed to update lobby: %w", err)
	}

	return nil
}

// IncrementLobbyPlayers increments current amount of players in the lobby.
func (r *Repository) IncrementLobbyPlayers(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "LobbyIncrementPlayers")
	defer span.End()

	key := lobbyPrefix + id
	cmd := r.valkey.B().Hincrby().Key(key).Field(lobbyCurrentPlayersField).Increment(1).Build()

	if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to increment current players: %w", err)
	}

	return nil
}

// DecrementLobbyPlayers decrements current amount of players in the lobby.
func (r *Repository) DecrementLobbyPlayers(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "LobbyDecrementPlayers")
	defer span.End()

	key := lobbyPrefix + id
	cmd := r.valkey.B().Hincrby().Key(key).Field(lobbyCurrentPlayersField).Increment(-1).Build()

	if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to decrement current players: %w", err)
	}

	return nil
}

// DeleteLobby deletes lobby from the database (key and from sorted set).
func (r *Repository) DeleteLobby(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "DeleteLobby")
	defer span.End()

	key := lobbyPrefix + id
	keyRemoveCmd := r.valkey.B().Del().Key(key).Build()

	if err := r.valkey.Do(ctx, keyRemoveCmd).Error(); err != nil {
		return fmt.Errorf("failed to delete lobby: %w", err)
	}

	sortedSetRemoveCmd := r.valkey.B().Zrem().Key(lobbiesPrefix).Member(id).Build()
	if err := r.valkey.Do(ctx, sortedSetRemoveCmd).Error(); err != nil {
		return fmt.Errorf("failed to remove lobby from sorted set: %w", err)
	}

	return nil
}

// AddLobbyExpiration sets expiration for the lobby (to remove empty lobbies).
func (r *Repository) AddLobbyExpiration(ctx context.Context, id string, ttl time.Duration) error {
	ctx, span := r.tracer.Start(ctx, "AddLobbyExpiration")
	defer span.End()

	key := lobbyPrefix + id
	keyCmd := r.valkey.B().Expire().Key(key).Seconds(int64(ttl.Seconds())).Build()

	if err := r.valkey.Do(ctx, keyCmd).Error(); err != nil {
		return fmt.Errorf("failed to set lobby expiration: %w", err)
	}

	return nil
}

// DeleteLobbyExpiration deletes expiration for the lobby (when someone connects to empty lobby).
func (r *Repository) DeleteLobbyExpiration(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "DeleteLobbyExpiration")
	defer span.End()

	key := lobbyPrefix + id
	cmd := r.valkey.B().Persist().Key(key).Build()

	if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to delete lobby expiration: %w", err)
	}

	return nil
}

// GetLobbies gets available lobbies (non-private) from the database using pagination.
func (r *Repository) GetLobbies(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error) {
	ctx, span := r.tracer.Start(ctx, "Lobbies")
	defer span.End()

	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize - 1 // Inclusive range

	// Get paginated lobby IDs from sorted set
	zrevrangeCmd := r.valkey.B().Zrevrange().
		Key(lobbiesPrefix).
		Start(int64(start)).
		Stop(int64(end)).
		Build()

	lobbyIDs, err := r.valkey.Do(ctx, zrevrangeCmd).AsStrSlice()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get lobby IDs: %w", err)
	}

	if len(lobbyIDs) == 0 {
		return []lobby.Lobby{}, 0, nil
	}

	// Pipeline HGETALL commands for all paginated lobbies
	getLobbyCmds := make(valkey.Commands, len(lobbyIDs))
	for i, id := range lobbyIDs {
		getLobbyCmds[i] = r.valkey.B().Hgetall().Key(lobbyPrefix + id).Build()
	}

	lobbyResults := r.valkey.DoMulti(ctx, getLobbyCmds...)
	lobbies := make([]lobby.Lobby, 0, len(lobbyResults))

	for i, result := range lobbyResults {
		resp, err := result.AsStrMap()
		if err != nil || len(resp) == 0 {
			slog.Debug("error getting lobby map",
				slog.String("lobbyID", lobbyIDs[i]),
				slog.Any("error", err))

			continue
		}

		lobby, err := parseLobbyData(lobbyIDs[i], resp)
		if err != nil {
			slog.Debug("error parsing lobby",
				slog.String("lobbyID", lobbyIDs[i]),
				slog.Any("error", err))

			continue
		}

		lobbies = append(lobbies, lobby)
	}

	totalCmd := r.valkey.B().Zcard().Key(lobbiesPrefix).Build()
	total, _ := r.valkey.Do(ctx, totalCmd).AsInt64()

	return lobbies, int(total), nil
}

func parseLobbyData(id string, data map[string]string) (lobby.Lobby, error) {
	creatorID, err := strconv.Atoi(data[lobbyCreatorIDField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid creator_id: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, data[lobbyCreatedAtField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid created_at: %w", err)
	}

	rounds, err := strconv.Atoi(data[lobbyRoundsField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid rounds: %w", err)
	}

	timerSeconds, err := strconv.Atoi(data[lobbyTimerSecondsField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid timer_seconds: %w", err)
	}

	movementAllowed, err := strconv.ParseBool(data[lobbyMovementAllowedField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid movement_allowed: %w", err)
	}

	currentPlayers, err := strconv.Atoi(data[lobbyCurrentPlayersField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid current_players: %w", err)
	}

	maxPlayers, err := strconv.Atoi(data[lobbyMaxPlayersField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid max_players: %w", err)
	}

	return lobby.Lobby{
		ID:              id,
		CreatorID:       creatorID,
		CreatedAt:       createdAt,
		Rounds:          rounds,
		Provider:        data[lobbyProviderField],
		TimerSeconds:    timerSeconds,
		MaxPlayers:      maxPlayers,
		MovementAllowed: movementAllowed,
		CurrentPlayers:  currentPlayers,
	}, nil
}

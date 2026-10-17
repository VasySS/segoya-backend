package valkey

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

const (
	lobbyPrefix               = "lobby:"
	lobbyPrivatePrefix        = "lobby:private:" // different prefix to exclude private lobbies from GetLobbies pagination
	lobbiesPrefix             = "lobbies:sorted"
	lobbyIDField              = "id"
	lobbyPrivateField         = "private"
	lobbyCreatorIDField       = "creatorID"
	lobbyCreatedAtField       = "createdAt"
	lobbyRoundsField          = "rounds"
	lobbyProviderField        = "provider"
	lobbyTimerSecondsField    = "timerSeconds"
	lobbyMovementAllowedField = "movementAllowed"
	lobbyMaxPlayersField      = "maxPlayers"
	lobbyCurrentPlayersField  = "currentPlayers"
)

// NewLobby atomically creates an empty public lobby and its expiration.
func (r *Repository) NewLobby(ctx context.Context, req dto.NewLobbyRequestDB) error {
	return r.createLobby(ctx, req, false)
}

// NewPrivateLobby atomically creates an empty private lobby and its expiration.
func (r *Repository) NewPrivateLobby(ctx context.Context, req dto.NewLobbyRequestDB) error {
	return r.createLobby(ctx, req, true)
}

func (r *Repository) createLobby(ctx context.Context, req dto.NewLobbyRequestDB, private bool) error {
	value := lobby.Lobby{
		Provider:     req.Provider,
		Rounds:       req.Rounds,
		TimerSeconds: req.TimerSeconds,
		MaxPlayers:   req.MaxPlayers,
	}
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validate game settings and state: %w", err)
	}

	ttl := req.Expiration
	if ttl <= 0 {
		ttl = 3 * time.Minute
	}

	key := lobbyPrefix + req.ID
	if private {
		key = lobbyPrivatePrefix + req.ID
	}

	fields := map[string]string{
		lobbyIDField:              req.ID,
		lobbyCreatorIDField:       req.CreatorID.String(),
		lobbyCreatedAtField:       req.RequestTime.Format(time.RFC3339Nano),
		lobbyRoundsField:          strconv.Itoa(req.Rounds),
		lobbyProviderField:        req.Provider,
		lobbyTimerSecondsField:    strconv.Itoa(req.TimerSeconds),
		lobbyMovementAllowedField: strconv.FormatBool(req.MovementAllowed),
		lobbyMaxPlayersField:      strconv.Itoa(req.MaxPlayers),
		lobbyCurrentPlayersField:  "0", lobbyPrivateField: strconv.FormatBool(private),
	}

	return r.runLobbyTransaction(ctx, req.ID, func(client valkey.DedicatedClient) (valkey.Commands, error) {
		exists, err := client.Do(ctx, client.B().Exists().Key(key).Build()).AsBool()
		if err != nil {
			return nil, fmt.Errorf("check lobby existence: %w", err)
		}

		if exists {
			return nil, errLobbyAlreadyExists
		}

		setFields := client.B().Hset().Key(key).FieldValue()
		for field, value := range fields {
			setFields = setFields.FieldValue(field, value)
		}

		commands := valkey.Commands{
			setFields.Build(),
			client.B().Pexpire().Key(key).Milliseconds(ttl.Milliseconds()).Build(),
		}
		if !private {
			commands = append(commands, client.B().Zadd().Key(lobbiesPrefix).
				ScoreMember().ScoreMember(float64(req.RequestTime.Unix()), req.ID).Build())
		}

		return commands, nil
	})
}

// GetLobby gets lobby from the database (public or private).
func (r *Repository) GetLobby(ctx context.Context, id string) (lobby.Lobby, error) {
	ctx, span := r.tracer.Start(ctx, "GetLobby")
	defer span.End()

	key, err := r.getLobbyKey(ctx, id)
	if err != nil {
		return lobby.Lobby{}, err
	}

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
func (r *Repository) UpdateLobby(ctx context.Context, info lobby.Lobby) error {
	if err := info.Validate(); err != nil {
		return fmt.Errorf("validate game settings and state: %w", err)
	}

	return r.runLobbyTransaction(ctx, info.ID, func(client valkey.DedicatedClient) (valkey.Commands, error) {
		key, data, err := readWatchedLobby(ctx, client, info.ID)
		if err != nil {
			return nil, err
		}

		if data[lobbyCreatorIDField] != info.CreatorID.String() {
			return nil, lobby.ErrOnlyCreatorCanUpdateSettings
		}

		return valkey.Commands{
			client.B().Hset().Key(key).FieldValue().
				FieldValue(lobbyRoundsField, strconv.Itoa(info.Rounds)).
				FieldValue(lobbyProviderField, info.Provider).
				FieldValue(lobbyTimerSecondsField, strconv.Itoa(info.TimerSeconds)).
				FieldValue(lobbyMovementAllowedField, strconv.FormatBool(info.MovementAllowed)).Build(),
		}, nil
	})
}

// DeleteLobby deletes lobby from the database (key and from sorted set).
func (r *Repository) DeleteLobby(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "DeleteLobby")
	defer span.End()

	key, err := r.getLobbyKey(ctx, id)
	if err != nil {
		return err
	}

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

// GetLobbies gets available lobbies (non-private) from the database using pagination.
func (r *Repository) GetLobbies(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error) {
	ctx, span := r.tracer.Start(ctx, "GetLobbies")
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

// getLobbyKey returns the key (public or private) for the lobby based on its ID.
func (r *Repository) getLobbyKey(ctx context.Context, id string) (string, error) {
	publicLobbyKey := lobbyPrefix + id
	privateLobbyKey := lobbyPrivatePrefix + id

	publicExists, err := r.valkey.Do(ctx, r.valkey.B().Exists().Key(publicLobbyKey).Build()).AsBool()
	if err != nil {
		return "", fmt.Errorf("failed to check if public lobby exists: %w", err)
	}

	// check if public lobby exists, if not, it's private
	if publicExists {
		return publicLobbyKey, nil
	}

	return privateLobbyKey, nil
}

func parseLobbyData(id string, data map[string]string) (lobby.Lobby, error) {
	creatorID, err := uuid.Parse(data[lobbyCreatorIDField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid creatorID: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, data[lobbyCreatedAtField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid createdAt: %w", err)
	}

	rounds, err := strconv.Atoi(data[lobbyRoundsField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid rounds: %w", err)
	}

	timerSeconds, err := strconv.Atoi(data[lobbyTimerSecondsField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid timerSeconds: %w", err)
	}

	movementAllowed, err := strconv.ParseBool(data[lobbyMovementAllowedField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid movementAllowed: %w", err)
	}

	currentPlayers, err := strconv.Atoi(data[lobbyCurrentPlayersField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid currentPlayers: %w", err)
	}

	maxPlayers, err := strconv.Atoi(data[lobbyMaxPlayersField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid maxPlayers: %w", err)
	}

	private, err := strconv.ParseBool(data[lobbyPrivateField])
	if err != nil {
		return lobby.Lobby{}, fmt.Errorf("invalid private: %w", err)
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
		Private:         private,
	}, nil
}

package valkey

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

var errLobbyAlreadyExists = errors.New("lobby already exists")

func lobbyKeys(id string) []string {
	return []string{lobbyPrefix + id, lobbyPrivatePrefix + id}
}

// runLobbyTransaction retries only when EXEC aborts because a watched key changed.
// prepare reads the watched state and builds writes without performing side effects.
func (r *Repository) runLobbyTransaction(
	ctx context.Context,
	id string,
	prepare func(valkey.DedicatedClient) (valkey.Commands, error),
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start lobby transaction: %w", err)
	}

	err := r.valkey.Dedicated(func(client valkey.DedicatedClient) error {
		defer func() {
			// Early returns must not leave WATCH state on a pooled connection.
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()

			status, err := client.Do(cleanupCtx, client.B().Unwatch().Build()).ToString()
			if err != nil || status != "OK" {
				client.Close()
			}
		}()

		for {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("retry lobby transaction: %w", err)
			}

			if err := client.Do(ctx, client.B().Watch().Key(lobbyKeys(id)...).Build()).Error(); err != nil {
				return fmt.Errorf("watch lobby: %w", err)
			}

			commands, err := prepare(client)
			if err != nil || len(commands) == 0 {
				return err
			}

			committed, err := execLobbyTransaction(ctx, client, commands)
			if err != nil {
				return err
			}

			if committed {
				return nil
			}
		}
	})
	if err != nil {
		return fmt.Errorf("lobby transaction: %w", err)
	}

	return nil
}

func execLobbyTransaction(
	ctx context.Context,
	client valkey.DedicatedClient,
	commands valkey.Commands,
) (bool, error) {
	transaction := make(valkey.Commands, 0, len(commands)+2)
	transaction = append(transaction, client.B().Multi().Build())
	transaction = append(transaction, commands...)
	transaction = append(transaction, client.B().Exec().Build())
	results := client.DoMulti(ctx, transaction...)

	for _, result := range results[:len(results)-1] {
		if err := result.Error(); err != nil {
			return false, fmt.Errorf("queue lobby transaction: %w", err)
		}
	}

	replies, err := results[len(results)-1].ToArray()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("execute lobby transaction: %w", err)
	}

	for _, reply := range replies {
		if err := reply.Error(); err != nil {
			return false, fmt.Errorf("write lobby state: %w", err)
		}
	}

	return true, nil
}

// readWatchedLobby uses the same connection that watches both possible lobby keys.
func readWatchedLobby(
	ctx context.Context,
	client valkey.DedicatedClient,
	id string,
) (string, map[string]string, error) {
	for _, key := range lobbyKeys(id) {
		data, err := client.Do(ctx, client.B().Hgetall().Key(key).Build()).AsStrMap()
		if err != nil {
			return "", nil, fmt.Errorf("read watched lobby: %w", err)
		}

		if len(data) != 0 {
			return key, data, nil
		}
	}

	return "", nil, lobby.ErrNotFound
}

// AdmitLobbyUser reserves one slot per user and session, removing the empty-lobby TTL atomically.
func (r *Repository) AdmitLobbyUser(ctx context.Context, id string, userID uuid.UUID, sessionID string) error {
	if userID == uuid.Nil || sessionID == "" {
		return lobby.ErrInvalidPlayers
	}

	return r.runLobbyTransaction(ctx, id, func(client valkey.DedicatedClient) (valkey.Commands, error) {
		key, data, err := readWatchedLobby(ctx, client, id)
		if err != nil {
			return nil, err
		}

		field := "member:" + userID.String()
		if token, admitted := data[field]; admitted {
			if token == sessionID {
				return nil, nil
			}

			return nil, lobby.ErrAlreadyConnected
		}

		count, err := strconv.Atoi(data[lobbyCurrentPlayersField])
		if err != nil || count < 0 {
			return nil, lobby.ErrInvalidPlayers
		}

		maximum, err := strconv.Atoi(data[lobbyMaxPlayersField])
		if err != nil || maximum < 2 || maximum > 10 {
			return nil, lobby.ErrInvalidPlayers
		}

		if count >= maximum {
			return nil, lobby.ErrLobbyIsFull
		}

		return valkey.Commands{
			client.B().Hset().Key(key).FieldValue().
				FieldValue(field, sessionID).
				FieldValue(lobbyCurrentPlayersField, strconv.Itoa(count+1)).Build(),
			client.B().Persist().Key(key).Build(),
		}, nil
	})
}

// ReleaseLobbyUser releases only the matching reservation; retries and stale sockets are harmless.
func (r *Repository) ReleaseLobbyUser(
	ctx context.Context,
	id string,
	userID uuid.UUID,
	sessionID string,
	ttl time.Duration,
) error {
	if ttl <= 0 {
		ttl = 3 * time.Minute
	}

	return r.runLobbyTransaction(ctx, id, func(client valkey.DedicatedClient) (valkey.Commands, error) {
		key, data, err := readWatchedLobby(ctx, client, id)
		if errors.Is(err, lobby.ErrNotFound) {
			return nil, nil
		}

		if err != nil {
			return nil, err
		}

		field := "member:" + userID.String()
		if token, admitted := data[field]; !admitted || token != sessionID {
			return nil, nil
		}

		count, err := strconv.Atoi(data[lobbyCurrentPlayersField])
		if err != nil {
			return nil, lobby.ErrInvalidPlayers
		}

		var remaining int

		if count > 0 {
			remaining = count - 1
		}

		commands := valkey.Commands{
			client.B().Hdel().Key(key).Field(field).Build(),
			client.B().Hset().Key(key).FieldValue().
				FieldValue(lobbyCurrentPlayersField, strconv.Itoa(remaining)).Build(),
		}
		if remaining == 0 {
			commands = append(commands, client.B().Pexpire().Key(key).Milliseconds(ttl.Milliseconds()).Build())
		} else {
			commands = append(commands, client.B().Persist().Key(key).Build())
		}

		return commands, nil
	})
}

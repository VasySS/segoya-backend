//go:build integration

//nolint:paralleltest // Serial container scenarios bound Docker resources and reuse fixture clients.
package integration_test

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	valkeyrepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
)

// These wrappers preserve the original deterministic WATCH conflict hook: a real
// second connection changes state after WATCH and reads, before MULTI/EXEC.
type lobbyTransactionClient struct {
	valkey.Client

	beforeExec func()
}

func (c lobbyTransactionClient) Dedicated(fn func(valkey.DedicatedClient) error) error {
	err := c.Client.Dedicated(func(client valkey.DedicatedClient) error {
		return fn(lobbyTransactionConnection{DedicatedClient: client, beforeExec: c.beforeExec})
	})
	if err != nil {
		return fmt.Errorf("dedicated lobby transaction: %w", err)
	}

	return nil
}

type lobbyTransactionConnection struct {
	valkey.DedicatedClient

	beforeExec func()
}

func (c lobbyTransactionConnection) DoMulti(ctx context.Context, commands ...valkey.Completed) []valkey.ValkeyResult {
	c.beforeExec()
	return c.DedicatedClient.DoMulti(ctx, commands...)
}

func TestLobbyWatchRetriesAdmission(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		var attempts atomic.Int32

		retrying := valkeyrepo.New(f.ctx, lobbyTransactionClient{
			Client: f.client,
			beforeExec: func() {
				if attempts.Add(1) == 1 {
					require.NoError(t, f.client.Do(f.ctx, f.client.B().Hset().Key(f.key).FieldValue().
						FieldValue("movementAllowed", "true").Build()).Error())
				}
			},
		})
		require.NoError(t, retrying.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))
		require.EqualValues(t, 2, attempts.Load())
		f.assertMembers(t, 1)
		value, err := f.repo.GetLobby(f.ctx, f.id)
		require.NoError(t, err)
		require.True(t, value.MovementAllowed)
		require.EqualValues(t, -1, f.ttl(t))
	})
}

func TestLobbyWatchRetryRechecksCapacity(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))

		competitor, rejected := uuid.New(), uuid.New()

		var attempts atomic.Int32

		retrying := valkeyrepo.New(f.ctx, lobbyTransactionClient{
			Client: f.client,
			beforeExec: func() {
				if attempts.Add(1) == 1 {
					require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, competitor, "competitor"))
				}
			},
		})
		require.ErrorIs(t, retrying.AdmitLobbyUser(f.ctx, f.id, rejected, "rejected"), lobby.ErrLobbyIsFull)
		require.EqualValues(t, 1, attempts.Load())
		f.assertMembers(t, 2)
		memberExists, err := f.client.Do(f.ctx, f.client.B().Hexists().Key(f.key).
			Field("member:"+rejected.String()).Build()).AsBool()
		require.NoError(t, err)
		require.False(t, memberExists)
	})
}

func TestLobbyWatchRetryPreservesConcurrentMembership(t *testing.T) {
	for _, operation := range []string{"settings", "release"} {
		t.Run(operation, func(t *testing.T) {
			withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
				t.Helper()

				require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))

				var attempts atomic.Int32

				other := uuid.New()
				retrying := valkeyrepo.New(f.ctx, lobbyTransactionClient{
					Client: f.client,
					beforeExec: func() {
						if attempts.Add(1) == 1 {
							require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, other, "other"))
						}
					},
				})
				expectedMembers := 1

				if operation == "settings" {
					value, err := f.repo.GetLobby(f.ctx, f.id)
					require.NoError(t, err)

					value.Provider, value.Rounds = "yandex", 5
					require.NoError(t, retrying.UpdateLobby(f.ctx, value))
					stored, err := f.repo.GetLobby(f.ctx, f.id)
					require.NoError(t, err)
					require.Equal(t, "yandex", stored.Provider)
					require.Equal(t, 5, stored.Rounds)

					expectedMembers = 2
				} else {
					require.NoError(t, retrying.ReleaseLobbyUser(f.ctx, f.id, f.creator, "creator", time.Minute))
				}

				require.EqualValues(t, 2, attempts.Load())
				f.assertMembers(t, expectedMembers)
				require.EqualValues(t, -1, f.ttl(t))
			})
		})
	}
}

func TestLobbyWatchRetriesStopOnCancellation(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		ctx, cancel := context.WithCancel(f.ctx)
		defer cancel()

		var attempts atomic.Int32

		retrying := valkeyrepo.New(f.ctx, lobbyTransactionClient{
			Client: f.client,
			beforeExec: func() {
				attempt := attempts.Add(1)
				require.NoError(t, f.client.Do(f.ctx, f.client.B().Hset().Key(f.key).FieldValue().
					FieldValue("conflict", strconv.Itoa(int(attempt))).Build()).Error())

				if attempt == 3 {
					cancel()
				}
			},
		})
		require.ErrorIs(t, retrying.AdmitLobbyUser(ctx, f.id, f.creator, "canceled"), context.Canceled)
		require.EqualValues(t, 3, attempts.Load())
		f.assertMembers(t, 0)
		require.Positive(t, f.ttl(t))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "after-cancellation"))
		f.assertMembers(t, 1)
	})
}

// Reusing an explicitly pinned connection makes leaked WATCH state observable.
// The production repository still owns UNWATCH on each early-return path.
type pinnedLobbyClient struct {
	valkey.Client

	connection valkey.DedicatedClient
}

func (c pinnedLobbyClient) Dedicated(fn func(valkey.DedicatedClient) error) error {
	if err := fn(c.connection); err != nil {
		return fmt.Errorf("pinned transaction: %w", err)
	}

	return nil
}

func TestLobbyEarlyReturnsClearWatch(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, uuid.New(), "other"))

		connection, release := f.client.Dedicate()
		defer release()

		repo := valkeyrepo.New(f.ctx, pinnedLobbyClient{Client: f.client, connection: connection})
		missingID := f.id + "-missing"

		for _, tc := range []struct {
			name     string
			run      func() error
			expected error
		}{
			{
				name: "same session",
				run:  func() error { return repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator") },
			},
			{
				name: "stale release",
				run:  func() error { return repo.ReleaseLobbyUser(f.ctx, f.id, f.creator, "stale", time.Minute) },
			},
			{
				name: "already connected", expected: lobby.ErrAlreadyConnected,
				run: func() error { return repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "duplicate") },
			},
			{
				name: "full lobby", expected: lobby.ErrLobbyIsFull,
				run: func() error { return repo.AdmitLobbyUser(f.ctx, f.id, uuid.New(), "extra") },
			},
			{
				name: "missing lobby", expected: lobby.ErrNotFound,
				run: func() error { return repo.AdmitLobbyUser(f.ctx, missingID, uuid.New(), "extra") },
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.run()
				if tc.expected != nil {
					require.ErrorIs(t, err, tc.expected)
				} else {
					require.NoError(t, err)
				}
				// Mutating the formerly watched key must not abort this unrelated transaction.
				watched := f.key
				if tc.name == "missing lobby" {
					watched = "lobby:" + missingID
				}

				require.NoError(t, f.client.Do(f.ctx, f.client.B().Hset().Key(watched).FieldValue().
					FieldValue("probe", tc.name).Build()).Error())

				results := connection.DoMulti(f.ctx,
					connection.B().Multi().Build(),
					connection.B().Set().Key("watch-cleanup-probe").Value(tc.name).Build(),
					connection.B().Exec().Build(),
				)
				require.Len(t, results, 3)
				replies, err := results[2].ToArray()
				require.NoError(t, err, "early return must release WATCH state")
				require.Len(t, replies, 1)
				require.NoError(t, replies[0].Error())
			})
		}

		f.assertMembers(t, 2)
	})
}

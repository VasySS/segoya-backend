//go:build integration

//nolint:paralleltest // Serial container scenarios bound Docker resources and reuse fixture clients.
package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

func runLobbyRace(t *testing.T, f lobbyFixture, operations ...func() error) []error {
	t.Helper()

	ready := make(chan struct{})

	results := make(chan error, len(operations))

	for _, operation := range operations {
		f.workers.Go(func() {
			select {
			case <-ready:
				results <- operation()
			case <-f.ctx.Done():
				results <- f.ctx.Err()
			}
		})
	}

	close(ready)

	errors := make([]error, 0, len(operations))
	for range operations {
		errors = append(errors, receive(t, f.ctx, results))
	}

	return errors
}

func TestLobbyAdmissionAndStaleRelease(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		require.Positive(t, f.ttl(t))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "original"))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "original"))
		require.ErrorIs(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "different"), lobby.ErrAlreadyConnected)
		f.assertMembers(t, 1)
		require.EqualValues(t, -1, f.ttl(t))
		require.NoError(t, f.repo.ReleaseLobbyUser(f.ctx, f.id, f.creator, "original", time.Minute))
		require.NoError(t, f.repo.ReleaseLobbyUser(f.ctx, f.id, f.creator, "original", time.Minute))
		f.assertMembers(t, 0)
		require.Positive(t, f.ttl(t))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "replacement"))
		require.NoError(t, f.repo.ReleaseLobbyUser(f.ctx, f.id, f.creator, "original", time.Minute))
		f.assertMembers(t, 1)
		require.EqualValues(t, -1, f.ttl(t))
	})
}

func TestLobbyConcurrentAdmissionRespectsCapacity(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))

		first, second := uuid.New(), uuid.New()
		results := runLobbyRace(t, f,
			func() error { return f.repo.AdmitLobbyUser(f.ctx, f.id, first, "first") },
			func() error { return f.repo.AdmitLobbyUser(f.ctx, f.id, second, "second") },
		)
		joined, full := 0, 0

		for _, err := range results {
			if err == nil {
				joined++
			} else {
				require.ErrorIs(t, err, lobby.ErrLobbyIsFull)

				full++
			}
		}

		require.Equal(t, 1, joined)
		require.Equal(t, 1, full)
		f.assertMembers(t, 2)
		require.EqualValues(t, -1, f.ttl(t))
	})
}

func TestLobbyConcurrentDeparturesRestoreExpiration(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		other := uuid.New()

		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, other, "other"))

		for _, err := range runLobbyRace(t, f,
			func() error { return f.repo.ReleaseLobbyUser(f.ctx, f.id, f.creator, "creator", time.Minute) },
			func() error { return f.repo.ReleaseLobbyUser(f.ctx, f.id, other, "other", time.Minute) },
			func() error { return f.repo.ReleaseLobbyUser(f.ctx, f.id, other, "other", time.Minute) },
		) {
			require.NoError(t, err)
		}

		f.assertMembers(t, 0)
		require.Positive(t, f.ttl(t))
		require.LessOrEqual(t, f.ttl(t), int64(60_000))
	})
}

func TestLobbyJoinRacingLastReleaseKeepsLiveLobby(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		joining := uuid.New()

		for iteration := range 10 {
			t.Run(fmt.Sprintf("attempt %d", iteration+1), func(t *testing.T) {
				require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "creator"))

				for _, err := range runLobbyRace(t, f,
					func() error { return f.repo.ReleaseLobbyUser(f.ctx, f.id, f.creator, "creator", time.Minute) },
					func() error { return f.repo.AdmitLobbyUser(f.ctx, f.id, joining, "joining") },
				) {
					require.NoError(t, err)
				}

				f.assertMembers(t, 1)
				require.EqualValues(t, -1, f.ttl(t))
				require.NoError(t, f.repo.ReleaseLobbyUser(f.ctx, f.id, joining, "joining", time.Minute))
			})
		}
	})
}

func TestLobbyCanceledAdmissionDoesNotMutateState(t *testing.T) {
	withLobbyKinds(t, func(t *testing.T, f lobbyFixture) {
		t.Helper()

		ctx, cancel := context.WithCancel(f.ctx)
		cancel()
		require.ErrorIs(t, f.repo.AdmitLobbyUser(ctx, f.id, f.creator, "canceled"), context.Canceled)
		f.assertMembers(t, 0)
		require.Positive(t, f.ttl(t))
		require.NoError(t, f.repo.AdmitLobbyUser(f.ctx, f.id, f.creator, "valid"))
		f.assertMembers(t, 1)
	})
}

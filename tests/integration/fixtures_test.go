//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/dto"
	valkeyrepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
	"github.com/VasySS/segoya-backend/tests/containers"
)

type testWorkers struct {
	ctx  context.Context
	done []<-chan struct{}
}

func integrationTime() time.Time {
	return time.Date(2026, time.January, 2, 12, 0, 0, 123456000, time.UTC)
}

func newTestWorkers(t *testing.T) *testWorkers {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	workers := &testWorkers{ctx: ctx}

	t.Cleanup(func() {
		cancel()

		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()

		for _, done := range workers.done {
			select {
			case <-done:
			case <-cleanup.Done():
				t.Error("test worker did not exit after cancellation")
				return
			}
		}
	})

	return workers
}

func (w *testWorkers) Go(fn func()) {
	done := make(chan struct{})

	w.done = append(w.done, done)

	go func() {
		defer close(done)

		fn()
	}()
}

//nolint:ireturn // A typed result preserves each worker's return value.
func receive[T any](t *testing.T, ctx context.Context, result <-chan T) T {
	t.Helper()

	select {
	case value := <-result:
		return value
	case <-ctx.Done():
		t.Fatalf("waiting for test result: %v", ctx.Err())

		var zero T

		return zero
	}
}

type lobbyFixture struct {
	ctx     context.Context
	workers *testWorkers
	client  valkey.Client
	repo    *valkeyrepo.Repository
	id      string
	creator uuid.UUID
	key     string
}

func withLobbyKinds(t *testing.T, fn func(*testing.T, lobbyFixture)) {
	t.Helper()

	client := containers.NewTestValkey(t)

	for _, tc := range []struct {
		name    string
		private bool
	}{
		{name: "public"},
		{name: "private", private: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workers := newTestWorkers(t)
			repo := valkeyrepo.New(workers.ctx, client)
			id, creator := uuid.NewString(), uuid.New()
			req := dto.NewLobbyRequestDB{
				ID: id, CreatorID: creator, RequestTime: time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC),
				Provider: "google", Rounds: 3, TimerSeconds: 30, MaxPlayers: 2,
			}

			key := "lobby:" + id
			if tc.private {
				key = "lobby:private:" + id

				require.NoError(t, repo.NewPrivateLobby(workers.ctx, req))
			} else {
				require.NoError(t, repo.NewLobby(workers.ctx, req))
			}

			fn(t, lobbyFixture{
				ctx: workers.ctx, workers: workers, client: client, repo: repo,
				id: id, creator: creator, key: key,
			})
		})
	}
}

func (f lobbyFixture) ttl(t *testing.T) int64 {
	t.Helper()

	ttl, err := f.client.Do(f.ctx, f.client.B().Pttl().Key(f.key).Build()).AsInt64()
	require.NoError(t, err)

	return ttl
}

func (f lobbyFixture) assertMembers(t *testing.T, expected int) {
	t.Helper()

	value, err := f.repo.GetLobby(f.ctx, f.id)
	require.NoError(t, err)
	require.Equal(t, expected, value.CurrentPlayers)

	data, err := f.client.Do(f.ctx, f.client.B().Hgetall().Key(f.key).Build()).AsStrMap()
	require.NoError(t, err)

	members := 0

	for key := range data {
		if len(key) >= len("member:") && key[:len("member:")] == "member:" {
			members++
		}
	}

	require.Equal(t, expected, members, "member fields and count must agree")
}

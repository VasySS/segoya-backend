//go:build integration

package containers

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
)

// TestContext bounds fixture setup and repository operations to one minute.
func TestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	return ctx
}

// NewTestPostgres registers cleanup before the caller opens database connections.
func NewTestPostgres(t *testing.T) *PostgresContainer {
	t.Helper()
	container, err := NewPostgresContainer(TestContext(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		require.NoError(t, container.Terminate(ctx))
	})

	return container
}

// NewTestPostgresPool closes all connections before earlier fixture cleanups run.
func NewTestPostgresPool(t *testing.T, connectionString string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(TestContext(t), connectionString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

// NewTestValkey closes the client before terminating its isolated container.
//
//nolint:ireturn // Valkey's constructor exposes its client interface.
func NewTestValkey(t *testing.T) valkey.Client {
	t.Helper()
	container, err := NewValkeyContainer(TestContext(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		require.NoError(t, container.Terminate(ctx))
	})

	client, err := valkey.NewClient(valkey.MustParseURL(container.ConnectionString))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	return client
}

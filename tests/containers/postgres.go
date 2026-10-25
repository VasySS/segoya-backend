package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer is a wrapper around the postgres test container.
type PostgresContainer struct {
	*postgres.PostgresContainer

	ConnectionString string
}

// NewPostgresContainer creates a new postgres test container.
func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	postgresContainer, err := postgres.Run(ctx,
		"postgis/postgis:18-3.6@sha256:e84455a058f60f06dd8c40e66a2c628b8fc113d9fca0896e1f308b25b897207f",
		postgres.WithDatabase("segoya_data"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run postgres container: %w", err)
	}

	connString, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get postgres connection string: %w", err)
	}

	return &PostgresContainer{
		PostgresContainer: postgresContainer,
		ConnectionString:  connString,
	}, nil
}

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
		"postgres:18.1-alpine3.23@sha256:aa6eb304ddb6dd26df23d05db4e5cb05af8951cda3e0dc57731b771e0ef4ab29",
		postgres.WithDatabase("segoya_data"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(10*time.Second),
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

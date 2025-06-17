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
		"postgres:18-beta1-alpine3.22@sha256:7f7f8a4d719a82301470b0c3f3a586ef343e9164e888a7f117f85ae5b7773c41",
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

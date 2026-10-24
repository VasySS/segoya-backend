//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	postgresRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func fixtureTime() time.Time { return time.Date(2026, time.January, 2, 12, 0, 0, 123456000, time.UTC) }

type postgresTestSuite struct {
	suite.Suite

	ctx               context.Context
	postgresContainer *containers.PostgresContainer
	pool              *pgxpool.Pool
	postgresRepo      *postgresRepo.Repository
	userSequence      int
}

func (s *postgresTestSuite) SetupSuite() {
	s.postgresContainer = containers.NewTestPostgres(s.T())
	pool := containers.NewTestPostgresPool(s.T(), s.postgresContainer.ConnectionString)
	ctx := containers.TestContext(s.T())
	s.Require().NoError(tables.RunGooseMigrations(ctx, pool, "up"))
	_, err := pool.Exec(ctx, `INSERT INTO panorama_location(id, provider, streetview_id, lat, lng) VALUES
		('00000000-0000-0000-0000-000000000001', 'google', NULL, 0, 0),
		('00000000-0000-0000-0000-000000000002', 'yandex', NULL, 55.75, 37.625),
		('00000000-0000-0000-0000-000000000003', 'yandex_air', 'air-panorama', -12.5, 0),
		('00000000-0000-0000-0000-000000000004', 'seznam', NULL, 0, -73.5)`)
	s.Require().NoError(err)
	pool.Close()
	s.Require().NoError(s.postgresContainer.Snapshot(ctx))
}

func (s *postgresTestSuite) SetupTest() {
	s.userSequence = 0
	s.ctx = containers.TestContext(s.T())
	s.Require().NoError(s.postgresContainer.Restore(s.ctx))
	s.pool = containers.NewTestPostgresPool(s.T(), s.postgresContainer.ConnectionString)
	s.postgresRepo = postgresRepo.New(postgresRepo.NewTxManager(s.pool))
}

func (s *postgresTestSuite) newUserRequest() dto.RegisterRequestDB {
	s.T().Helper()

	s.userSequence++

	return dto.RegisterRequestDB{
		RequestTime: fixtureTime(), Username: fmt.Sprintf("player-%d", s.userSequence),
		Name: "Test Player", Password: "test-password-hash",
	}
}

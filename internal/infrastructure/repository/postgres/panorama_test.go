package postgres_test

import (
	"context"
	"testing"

	postgresRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/migrations/data"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
)

func TestPanoramaTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PanoramaTestSuite))
}

type PanoramaTestSuite struct {
	suite.Suite
	ctx               context.Context
	postgresContainer *containers.PostgresContainer
	postgresRepo      *postgresRepo.Repository
}

func (s *PanoramaTestSuite) SetupSuite() {
	s.ctx = context.Background()

	postgresContainer, err := containers.NewPostgresContainer(s.ctx)
	s.Require().NoError(err)

	s.postgresContainer = postgresContainer

	migrationsPool, err := pgxpool.New(s.ctx, postgresContainer.ConnectionString)
	s.Require().NoError(err)

	err = tables.RunGooseMigrations(s.ctx, migrationsPool, "up")
	s.Require().NoError(err)

	err = data.MigrateAllCSVData(s.ctx, migrationsPool)
	s.Require().NoError(err)

	// testcontainers lib needs all connections to db to be closed
	migrationsPool.Close()

	// save db state with all migrations applied
	err = s.postgresContainer.Snapshot(s.ctx)
	s.Require().NoError(err)
}

func (s *PanoramaTestSuite) TearDownSuite() {
	err := s.postgresContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *PanoramaTestSuite) SetupTest() {
	// restore db state with migrations only
	err := s.postgresContainer.Restore(s.ctx)
	s.Require().NoError(err)

	pool, err := pgxpool.New(s.ctx, s.postgresContainer.ConnectionString)
	s.Require().NoError(err)

	txManger := postgresRepo.NewTxManager(pool)
	repo := postgresRepo.New(txManger)

	s.postgresRepo = repo
}

func (s *PanoramaTestSuite) TestRandomGoogleStreetview() {
	panorama, err := s.postgresRepo.RandomGoogleStreetview(s.ctx)
	s.Require().NoError(err)

	s.NotEmpty(panorama.ID)
	s.NotEmpty(panorama.Lat)
	s.NotEmpty(panorama.Lng)
}

func (s *PanoramaTestSuite) TestGoogleStreetviewByID() {
	original, err := s.postgresRepo.RandomGoogleStreetview(s.ctx)
	s.Require().NoError(err)

	fetched, err := s.postgresRepo.GetGoogleStreetview(s.ctx, original.ID)
	s.Require().NoError(err)

	s.InEpsilon(original.ID, fetched.ID, 0.01)
	s.InEpsilon(original.Lat, fetched.Lat, 0.01)
	s.InEpsilon(original.Lng, fetched.Lng, 0.01)
}

func (s *PanoramaTestSuite) TestRandomYandexStreetview() {
	panorama, err := s.postgresRepo.RandomYandexStreetview(s.ctx)
	s.Require().NoError(err)

	s.NotEmpty(panorama.ID)
	s.NotEmpty(panorama.Lat)
	s.NotEmpty(panorama.Lng)
}

func (s *PanoramaTestSuite) TestYandexStreetviewByID() {
	original, err := s.postgresRepo.RandomYandexStreetview(s.ctx)
	s.Require().NoError(err)

	fetched, err := s.postgresRepo.GetYandexStreetview(s.ctx, original.ID)
	s.Require().NoError(err)

	s.InEpsilon(original.Lat, fetched.Lat, 0.01)
	s.InEpsilon(original.Lng, fetched.Lng, 0.01)
}

func (s *PanoramaTestSuite) TestRandomYandexAirview() {
	panorama, err := s.postgresRepo.RandomYandexAirview(s.ctx)
	s.Require().NoError(err)

	s.NotEmpty(panorama.StreetviewID)
	s.NotEmpty(panorama.Lat)
	s.NotEmpty(panorama.Lng)
}

func (s *PanoramaTestSuite) TestYandexAirviewByID() {
	original, err := s.postgresRepo.RandomYandexAirview(s.ctx)
	s.Require().NoError(err)

	fetched, err := s.postgresRepo.GetYandexAirview(s.ctx, original.ID)
	s.Require().NoError(err)

	s.Equal(original.ID, fetched.ID)
	s.Equal(original.StreetviewID, fetched.StreetviewID)
	s.InEpsilon(original.Lat, fetched.Lat, 0.01)
	s.InEpsilon(original.Lng, fetched.Lng, 0.01)
}

func (s *PanoramaTestSuite) TestRandomSeznamStreetview() {
	panorama, err := s.postgresRepo.RandomSeznamStreetview(s.ctx)
	s.Require().NoError(err)

	s.NotEmpty(panorama.ID)
	s.NotEmpty(panorama.Lat)
	s.NotEmpty(panorama.Lng)
}

func (s *PanoramaTestSuite) TestSeznamStreetviewByID() {
	original, err := s.postgresRepo.RandomSeznamStreetview(s.ctx)
	s.Require().NoError(err)

	fetched, err := s.postgresRepo.GetSeznamStreetview(s.ctx, original.ID)
	s.Require().NoError(err)

	s.Equal(original.ID, fetched.ID)
	s.InDelta(original.Lat, fetched.Lat, 0.01)
	s.InDelta(original.Lng, fetched.Lng, 0.01)
}

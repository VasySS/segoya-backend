package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	postgresRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
)

func TestOAuthTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(OAuthTestSuite))
}

type OAuthTestSuite struct {
	suite.Suite
	ctx               context.Context
	postgresContainer *containers.PostgresContainer
	postgresRepo      *postgresRepo.Repository
}

func (s *OAuthTestSuite) SetupSuite() {
	s.ctx = context.Background()

	postgresContainer, err := containers.NewPostgresContainer(s.ctx)
	s.Require().NoError(err)

	s.postgresContainer = postgresContainer

	migrationsPool, err := pgxpool.New(s.ctx, postgresContainer.ConnectionString)
	s.Require().NoError(err)

	err = tables.RunGooseMigrations(s.ctx, migrationsPool, "up")
	s.Require().NoError(err)

	// testcontainers lib needs all connections to db to be closed
	migrationsPool.Close()

	// save db state with all migrations applied
	err = s.postgresContainer.Snapshot(s.ctx)
	s.Require().NoError(err)
}

func (s *OAuthTestSuite) TearDownSuite() {
	err := s.postgresContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *OAuthTestSuite) SetupTest() {
	// restore db state with migrations only
	err := s.postgresContainer.Restore(s.ctx)
	s.Require().NoError(err)

	pool, err := pgxpool.New(s.ctx, s.postgresContainer.ConnectionString)
	s.Require().NoError(err)

	txManger := postgresRepo.NewTxManager(pool)
	repo := postgresRepo.New(txManger)

	s.postgresRepo = repo
}

func (s *OAuthTestSuite) newTestUser() (dto.RegisterRequestDB, user.PrivateProfile) {
	newUserReq := dto.RegisterRequestDB{
		RequestTime: time.Now().UTC(),
		Username:    gofakeit.Username(),
		Name:        gofakeit.Name(),
		Password:    gofakeit.LetterN(60), // emulating bcrypt hash
	}

	err := s.postgresRepo.NewUser(s.ctx, newUserReq)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, newUserReq.Username)
	s.Require().NoError(err)

	return newUserReq, newUser
}

func (s *OAuthTestSuite) TestNewOAuth() {
	_, testUser := s.newTestUser()

	req := dto.NewOAuthRequestDB{
		RequestTime: time.Now().UTC(),
		OAuthID:     gofakeit.UUID(),
		UserID:      testUser.ID,
		Issuer:      "discord",
	}

	err := s.postgresRepo.NewOAuth(s.ctx, req)
	s.Require().NoError(err)

	oauthEntries, err := s.postgresRepo.GetOAuth(s.ctx, testUser.ID)
	s.Require().NoError(err)
	s.Require().Len(oauthEntries, 1)

	s.Equal(req.OAuthID, oauthEntries[0].OAuthID)
	s.Equal(req.UserID, oauthEntries[0].UserID)
	s.Equal(req.Issuer, oauthEntries[0].Issuer)
	s.WithinDuration(req.RequestTime, oauthEntries[0].CreatedAt, 5*time.Millisecond)
}

func (s *OAuthTestSuite) TestOAuthInfo() {
	_, testUser := s.newTestUser()

	for range 3 {
		req := dto.NewOAuthRequestDB{
			RequestTime: time.Now().UTC(),
			OAuthID:     gofakeit.UUID(),
			UserID:      testUser.ID,
			Issuer:      "discord",
		}

		err := s.postgresRepo.NewOAuth(s.ctx, req)
		s.Require().NoError(err)
	}

	oauthEntries, err := s.postgresRepo.GetOAuth(s.ctx, testUser.ID)
	s.Require().NoError(err)
	s.Require().Len(oauthEntries, 3)
}

func (s *OAuthTestSuite) TestDeleteOAuth() {
	_, testUser := s.newTestUser()

	req := dto.NewOAuthRequestDB{
		RequestTime: time.Now().UTC(),
		OAuthID:     gofakeit.UUID(),
		UserID:      testUser.ID,
		Issuer:      "discord",
	}

	err := s.postgresRepo.NewOAuth(s.ctx, req)
	s.Require().NoError(err)

	oauthEntries, err := s.postgresRepo.GetOAuth(s.ctx, testUser.ID)
	s.Require().NoError(err)
	s.Require().Len(oauthEntries, 1)

	deleteReq := dto.DeleteOAuthRequest{
		UserID: testUser.ID,
		Issuer: req.Issuer,
	}

	err = s.postgresRepo.DeleteOAuth(s.ctx, deleteReq)
	s.Require().NoError(err)

	oauthEntries, err = s.postgresRepo.GetOAuth(s.ctx, testUser.ID)
	s.Require().NoError(err)
	s.Require().Empty(oauthEntries)
}

func (s *OAuthTestSuite) TestUserByOAuthIssuer() {
	testUserReq, testUser := s.newTestUser()

	newOAuthReq := dto.NewOAuthRequestDB{
		RequestTime: time.Now().UTC(),
		OAuthID:     gofakeit.UUID(),
		UserID:      testUser.ID,
		Issuer:      "discord",
	}

	userReq := dto.GetUserByOAuthRequest{
		OAuthID: newOAuthReq.OAuthID,
		Issuer:  newOAuthReq.Issuer,
	}

	_, err := s.postgresRepo.GetUserByOAuth(s.ctx, userReq)
	s.Require().ErrorIs(err, user.ErrOAuthNotFound)

	err = s.postgresRepo.NewOAuth(s.ctx, newOAuthReq)
	s.Require().NoError(err)

	u, err := s.postgresRepo.GetUserByOAuth(s.ctx, userReq)
	s.Require().NoError(err)

	s.Equal(testUser.ID, u.ID)
	s.Equal(testUser.Username, u.Username)
	s.Equal(testUser.Name, u.Name)
	s.Equal(testUser.Password, u.Password)
	s.WithinDuration(testUserReq.RequestTime, u.RegisterDate, 5*time.Millisecond)
}

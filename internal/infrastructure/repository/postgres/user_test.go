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

func TestUserTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserTestSuite))
}

type UserTestSuite struct {
	suite.Suite
	ctx               context.Context
	postgresContainer *containers.PostgresContainer
	postgresRepo      *postgresRepo.Repository
}

func (s *UserTestSuite) SetupSuite() {
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

func (s *UserTestSuite) TearDownSuite() {
	err := s.postgresContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *UserTestSuite) SetupTest() {
	// restore db state with migrations only
	err := s.postgresContainer.Restore(s.ctx)
	s.Require().NoError(err)

	pool, err := pgxpool.New(s.ctx, s.postgresContainer.ConnectionString)
	s.Require().NoError(err)

	txManger := postgresRepo.NewTxManager(pool)
	repo := postgresRepo.New(txManger)

	s.postgresRepo = repo
}

func (s *UserTestSuite) TestNewUser() {
	req := dto.RegisterRequestDB{
		RequestTime: time.Now().UTC(),
		Username:    gofakeit.Username(),
		Name:        gofakeit.Name(),
		Password:    gofakeit.LetterN(60), // emulating bcrypt hash
	}

	err := s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	u, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)

	s.Equal(req.Username, u.Username)
	s.Equal(req.Name, u.Name)
	s.Equal(req.Password, u.Password)
	s.WithinDuration(req.RequestTime, u.RegisterDate, 5*time.Millisecond)
	s.Empty(u.AvatarHash)
	s.Empty(u.AvatarLastUpdate)

	err = s.postgresRepo.NewUser(s.ctx, req)
	s.Equal(user.ErrAlreadyExists, err)
}

func (s *UserTestSuite) TestGetUserByUsername() {
	req := dto.RegisterRequestDB{
		RequestTime: time.Now().UTC(),
		Username:    gofakeit.Username(),
		Name:        gofakeit.Name(),
		Password:    gofakeit.LetterN(60), // emulating bcrypt hash
	}

	_, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().Error(err)
	s.Equal(user.ErrUserNotFound, err)

	err = s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	u, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)

	s.Equal(req.Username, u.Username)
	s.Equal(req.Name, u.Name)
	s.Equal(req.Password, u.Password)
	s.WithinDuration(req.RequestTime, u.RegisterDate, 5*time.Millisecond)
	s.Empty(u.AvatarHash)
	s.Empty(u.AvatarLastUpdate)
}

func (s *UserTestSuite) TestGetUserByID() {
	_, err := s.postgresRepo.GetUserByID(s.ctx, 1111)
	s.Require().ErrorIs(err, user.ErrUserNotFound)

	req := dto.RegisterRequestDB{
		RequestTime: time.Now().UTC(),
		Username:    gofakeit.Username(),
		Name:        gofakeit.Name(),
		Password:    gofakeit.LetterN(60), // emulating bcrypt hash
	}

	err = s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)

	u, err := s.postgresRepo.GetUserByID(s.ctx, newUser.ID)
	s.Require().NoError(err)

	s.Equal(req.Username, u.Username)
	s.Equal(req.Name, u.Name)
	s.Equal(req.Password, u.Password)
	s.WithinDuration(req.RequestTime, u.RegisterDate, 5*time.Millisecond)
	s.Empty(u.AvatarHash)
	s.Empty(u.AvatarLastUpdate)
}

func (s *UserTestSuite) TestUpdateUser() {
	req := dto.RegisterRequestDB{
		RequestTime: time.Now().UTC(),
		Username:    gofakeit.Username(),
		Name:        gofakeit.Name(),
		Password:    gofakeit.LetterN(60), // emulating bcrypt hash
	}

	err := s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)
	s.Equal(req.Username, newUser.Username)
	s.Equal(req.Name, newUser.Name)

	updateReq := dto.UpdateUserRequest{
		UserID: newUser.ID,
		Name:   gofakeit.Name(),
	}

	err = s.postgresRepo.UpdateUser(s.ctx, updateReq)
	s.Require().NoError(err)

	updatedUser, err := s.postgresRepo.GetUserByID(s.ctx, newUser.ID)
	s.Require().NoError(err)
	s.Equal(req.Username, updatedUser.Username)
	s.Equal(updateReq.Name, updatedUser.Name)
}

func (s *UserTestSuite) TestUpdateAvatar() {
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
	s.Empty(newUser.AvatarHash)

	req := dto.UpdateAvatarRequestDB{
		RequestTime: time.Now().UTC(),
		UserID:      newUser.ID,
		AvatarHash:  gofakeit.UUID(),
	}

	err = s.postgresRepo.UpdateAvatar(s.ctx, req)
	s.Require().NoError(err)

	updatedUser, err := s.postgresRepo.GetUserByID(s.ctx, newUser.ID)
	s.Require().NoError(err)
	s.Equal(req.AvatarHash, updatedUser.AvatarHash)
	s.WithinDuration(req.RequestTime, updatedUser.AvatarLastUpdate, 5*time.Millisecond)
}

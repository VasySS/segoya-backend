//go:build integration

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestUserTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserTestSuite))
}

type UserTestSuite struct{ postgresTestSuite }

func (s *UserTestSuite) TestNewUser() {
	req := s.newUserRequest()

	err := s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	u, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)

	s.Equal(req.Username, u.Username)
	s.Equal(req.Name, u.Name)
	s.Equal(req.Password, u.Password)
	s.Equal(req.RequestTime, u.RegisterDate)
	s.Empty(u.AvatarHash)
	s.Empty(u.AvatarLastUpdate)

	err = s.postgresRepo.NewUser(s.ctx, req)
	s.Equal(user.ErrAlreadyExists, err)
}

func (s *UserTestSuite) TestGetUserByUsername() {
	req := s.newUserRequest()

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
	s.Equal(req.RequestTime, u.RegisterDate)
	s.Empty(u.AvatarHash)
	s.Empty(u.AvatarLastUpdate)
}

func (s *UserTestSuite) TestGetUserByID() {
	_, err := s.postgresRepo.GetUserByID(s.ctx, uuid.New())
	s.Require().ErrorIs(err, user.ErrUserNotFound)

	req := s.newUserRequest()

	err = s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)

	u, err := s.postgresRepo.GetUserByID(s.ctx, newUser.ID)
	s.Require().NoError(err)

	s.Equal(req.Username, u.Username)
	s.Equal(req.Name, u.Name)
	s.Equal(req.Password, u.Password)
	s.Equal(req.RequestTime, u.RegisterDate)
	s.Empty(u.AvatarHash)
	s.Empty(u.AvatarLastUpdate)
}

func (s *UserTestSuite) TestUpdateUser() {
	req := s.newUserRequest()

	err := s.postgresRepo.NewUser(s.ctx, req)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, req.Username)
	s.Require().NoError(err)
	s.Equal(req.Username, newUser.Username)
	s.Equal(req.Name, newUser.Name)

	updateReq := dto.UpdateUserRequest{
		UserID: newUser.ID,
		Name:   "Updated player",
	}

	err = s.postgresRepo.UpdateUser(s.ctx, updateReq)
	s.Require().NoError(err)

	updatedUser, err := s.postgresRepo.GetUserByID(s.ctx, newUser.ID)
	s.Require().NoError(err)
	s.Equal(req.Username, updatedUser.Username)
	s.Equal(updateReq.Name, updatedUser.Name)
}

func (s *UserTestSuite) TestUpdateAvatar() {
	newUserReq := s.newUserRequest()

	err := s.postgresRepo.NewUser(s.ctx, newUserReq)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, newUserReq.Username)
	s.Require().NoError(err)
	s.Empty(newUser.AvatarHash)

	req := dto.UpdateAvatarRequestDB{
		RequestTime: fixtureTime(),
		UserID:      newUser.ID,
		AvatarHash:  "updated-avatar-hash",
	}

	err = s.postgresRepo.UpdateAvatar(s.ctx, req)
	s.Require().NoError(err)

	updatedUser, err := s.postgresRepo.GetUserByID(s.ctx, newUser.ID)
	s.Require().NoError(err)
	s.Equal(req.AvatarHash, updatedUser.AvatarHash)
	s.Equal(req.RequestTime, updatedUser.AvatarLastUpdate)
}

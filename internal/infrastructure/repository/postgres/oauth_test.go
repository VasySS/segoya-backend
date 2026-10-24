//go:build integration

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestOAuthTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(OAuthTestSuite))
}

type OAuthTestSuite struct{ postgresTestSuite }

func (s *OAuthTestSuite) TestNewOAuth() {
	_, testUser := s.newTestUser()

	req := dto.NewOAuthRequestDB{
		RequestTime: fixtureTime(),
		OAuthID:     uuid.NewString(),
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
	s.Equal(req.RequestTime, oauthEntries[0].CreatedAt)
}

func (s *OAuthTestSuite) TestOAuthInfo() {
	_, testUser := s.newTestUser()

	for range 3 {
		req := dto.NewOAuthRequestDB{
			RequestTime: fixtureTime(),
			OAuthID:     uuid.NewString(),
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
		RequestTime: fixtureTime(),
		OAuthID:     uuid.NewString(),
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
		RequestTime: fixtureTime(),
		OAuthID:     uuid.NewString(),
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
	s.Equal(testUserReq.RequestTime, u.RegisterDate)
}

func (s *OAuthTestSuite) newTestUser() (dto.RegisterRequestDB, user.PrivateProfile) {
	newUserReq := s.newUserRequest()

	err := s.postgresRepo.NewUser(s.ctx, newUserReq)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, newUserReq.Username)
	s.Require().NoError(err)

	return newUserReq, newUser
}

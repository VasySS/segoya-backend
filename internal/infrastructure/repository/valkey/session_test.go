package valkey_test

import (
	"context"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	valkeyRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
	"github.com/VasySS/segoya-backend/tests/containers"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/suite"
	"github.com/valkey-io/valkey-go"
)

func TestSessionTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SessionTestSuite))
}

type SessionTestSuite struct {
	suite.Suite
	ctx             context.Context
	valkeyContainer *containers.ValkeyContainer
	valkeyRepo      *valkeyRepo.Repository
}

func (s *SessionTestSuite) SetupSuite() {
	s.ctx = context.Background()

	valkeyContainer, err := containers.NewValkeyContainer(s.ctx)
	s.Require().NoError(err)

	valkeyClient, err := valkey.NewClient(valkey.MustParseURL(valkeyContainer.ConnectionString))
	s.Require().NoError(err)

	repo := valkeyRepo.New(valkeyClient)

	s.valkeyContainer = valkeyContainer
	s.valkeyRepo = repo
}

func (s *SessionTestSuite) TearDownSuite() {
	err := s.valkeyContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *SessionTestSuite) TestNewUserSession() {
	req := dto.NewSessionRequest{
		RequestTime:  time.Now().UTC(),
		UserID:       gofakeit.IntRange(1, 100),
		SessionID:    gofakeit.UUID(),
		RefreshToken: gofakeit.UUID(),
		Expiration:   2 * time.Second,
		UA:           "Test User Agent",
	}

	err := s.valkeyRepo.NewSession(s.ctx, req)
	s.Require().NoError(err)

	session, err := s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().NoError(err)
	s.Require().Equal(req.UserID, session.UserID)
	s.Require().Equal(req.SessionID, session.SessionID)
	s.Require().Equal(req.RefreshToken, session.RefreshToken)
	s.Require().Equal(req.UA, session.UA)

	time.Sleep(3 * time.Second)

	_, err = s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().Error(err)
}

func (s *SessionTestSuite) TestGetUserSession() {
	req := dto.NewSessionRequest{
		RequestTime:  time.Now().UTC(),
		UserID:       gofakeit.IntRange(1, 100),
		SessionID:    gofakeit.UUID(),
		RefreshToken: gofakeit.UUID(),
		Expiration:   2 * time.Second,
		UA:           "Test User Agent",
	}

	err := s.valkeyRepo.NewSession(s.ctx, req)
	s.Require().NoError(err)

	session, err := s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().NoError(err)
	s.Require().Equal(req.UserID, session.UserID)
	s.Require().Equal(req.SessionID, session.SessionID)
	s.Require().Equal(req.RefreshToken, session.RefreshToken)
	s.Require().Equal(req.UA, session.UA)

	time.Sleep(3 * time.Second)

	_, err = s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().Error(err)
}

func (s *SessionTestSuite) TestGetUserSessions() {
	userID := gofakeit.IntRange(1, 100)
	newSessionIDs := make([]string, 0, 3)

	for range 3 {
		req := dto.NewSessionRequest{
			RequestTime:  time.Now().UTC(),
			UserID:       userID,
			SessionID:    gofakeit.UUID(),
			RefreshToken: gofakeit.UUID(),
			Expiration:   2 * time.Second,
			UA:           "Test User Agent",
		}

		err := s.valkeyRepo.NewSession(s.ctx, req)
		s.Require().NoError(err)

		newSessionIDs = append(newSessionIDs, req.SessionID)
	}

	sessions, err := s.valkeyRepo.GetSessions(s.ctx, userID)
	s.Require().NoError(err)

	getSessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		getSessionIDs = append(getSessionIDs, session.SessionID)
	}

	s.ElementsMatch(newSessionIDs, getSessionIDs)
}

func (s *SessionTestSuite) TestUpdateUserSession() {
	userID := gofakeit.IntRange(1, 100)
	sessionID := gofakeit.UUID()

	createReq := dto.NewSessionRequest{
		RequestTime:  time.Now().UTC(),
		UserID:       userID,
		SessionID:    sessionID,
		RefreshToken: gofakeit.UUID(),
		Expiration:   2 * time.Second,
		UA:           "Test User Agent",
	}

	updateReq := dto.UpdateSessionRequest{
		RequestTime:  time.Now().UTC(),
		UserID:       userID,
		SessionID:    sessionID,
		RefreshToken: gofakeit.UUID(),
		Expiration:   10 * time.Second,
	}

	err := s.valkeyRepo.NewSession(s.ctx, createReq)
	s.Require().NoError(err)

	err = s.valkeyRepo.UpdateSession(s.ctx, updateReq)
	s.Require().NoError(err)

	time.Sleep(3 * time.Second)

	session, err := s.valkeyRepo.GetSession(s.ctx, updateReq.UserID, updateReq.SessionID)
	s.Require().NoError(err)

	s.Equal(updateReq.UserID, session.UserID)
	s.Equal(updateReq.SessionID, session.SessionID)
	s.Equal(updateReq.RefreshToken, session.RefreshToken)
	s.Equal(createReq.UA, session.UA)
	s.WithinDuration(updateReq.RequestTime, session.LastActive, 1*time.Second)
}

func (s *SessionTestSuite) TestDeleteUserSession() {
	userID := gofakeit.IntRange(1, 100)
	sessionID := gofakeit.UUID()

	req := dto.NewSessionRequest{
		RequestTime:  time.Now().UTC(),
		UserID:       userID,
		SessionID:    sessionID,
		RefreshToken: gofakeit.UUID(),
		Expiration:   10 * time.Second,
		UA:           "Test User Agent",
	}

	err := s.valkeyRepo.NewSession(s.ctx, req)
	s.Require().NoError(err)

	l, err := s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().NoError(err)
	s.Equal(req.UserID, l.UserID)

	err = s.valkeyRepo.DeleteSession(s.ctx, req.UserID, req.SessionID)
	s.Require().NoError(err)

	_, err = s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().Error(err)
}

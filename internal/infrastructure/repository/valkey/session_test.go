//go:build integration

package valkey_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestSessionTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SessionTestSuite))
}

type SessionTestSuite struct{ valkeyTestSuite }

func (s *SessionTestSuite) TestSessionRoundTrip() {
	req := s.newSession(uuid.New())
	got, err := s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().NoError(err)
	s.Equal(user.Session{
		UserID: req.UserID, SessionID: req.SessionID, RefreshToken: req.RefreshToken,
		UA: req.UA, LastActive: req.RequestTime,
	}, got)

	_, err = s.valkeyRepo.GetSession(s.ctx, uuid.New(), req.SessionID)
	s.Require().ErrorIs(err, user.ErrSessionNotFound)
	_, err = s.valkeyRepo.GetSession(s.ctx, req.UserID, uuid.New())
	s.Require().ErrorIs(err, user.ErrSessionNotFound)
}

func (s *SessionTestSuite) TestGetSessionsIsolatesUsers() {
	userID := uuid.New()
	empty, err := s.valkeyRepo.GetSessions(s.ctx, userID)
	s.Require().NoError(err)
	s.Empty(empty)

	expected := make([]uuid.UUID, 0, 3)
	for range 3 {
		expected = append(expected, s.newSession(userID).SessionID)
	}

	s.newSession(uuid.New())
	got, err := s.valkeyRepo.GetSessions(s.ctx, userID)
	s.Require().NoError(err)

	actual := make([]uuid.UUID, 0, len(got))
	for _, session := range got {
		actual = append(actual, session.SessionID)
		s.Equal(userID, session.UserID)
	}

	s.ElementsMatch(expected, actual)
}

func (s *SessionTestSuite) TestUpdateSessionReplacesTokenAndExtendsTTL() {
	req := s.newSession(uuid.New())
	key := "user:" + req.UserID.String() + "session:" + req.SessionID.String()
	s.Require().NoError(s.client.Do(s.ctx, s.client.B().Expire().Key(key).Seconds(2).Build()).Error())

	update := dto.UpdateSessionRequest{
		UserID: req.UserID, SessionID: req.SessionID, RequestTime: fixtureTime().Add(time.Minute),
		RefreshToken: "replacement-refresh-token", Expiration: time.Minute,
	}
	s.Require().NoError(s.valkeyRepo.UpdateSession(s.ctx, update))
	got, err := s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
	s.Require().NoError(err)
	s.Equal(update.RefreshToken, got.RefreshToken)
	s.Equal(update.RequestTime, got.LastActive)
	s.Equal(req.UA, got.UA)
	s.Equal(req.UserID, got.UserID)
	s.Equal(req.SessionID, got.SessionID)
	s.Greater(s.ttl(key), int64(30_000))
	s.LessOrEqual(s.ttl(key), int64(60_000))
}

func (s *SessionTestSuite) TestDeleteSessionPreservesOtherSessions() {
	userID := uuid.New()
	deleted, retained := s.newSession(userID), s.newSession(userID)
	s.Require().NoError(s.valkeyRepo.DeleteSession(s.ctx, userID, deleted.SessionID))
	s.Require().NoError(s.valkeyRepo.DeleteSession(s.ctx, userID, deleted.SessionID))
	_, err := s.valkeyRepo.GetSession(s.ctx, userID, deleted.SessionID)
	s.Require().ErrorIs(err, user.ErrSessionNotFound)
	_, err = s.valkeyRepo.GetSession(s.ctx, userID, retained.SessionID)
	s.NoError(err)
}

func (s *SessionTestSuite) TestSessionExpiration() {
	req := dto.NewSessionRequest{
		RequestTime: fixtureTime(), UserID: uuid.New(), SessionID: uuid.New(),
		RefreshToken: uuid.NewString(), Expiration: time.Second,
	}
	s.Require().NoError(s.valkeyRepo.NewSession(s.ctx, req))
	s.Eventually(func() bool {
		_, err := s.valkeyRepo.GetSession(s.ctx, req.UserID, req.SessionID)
		return errors.Is(err, user.ErrSessionNotFound)
	}, 3*time.Second, 20*time.Millisecond)

	sessions, err := s.valkeyRepo.GetSessions(s.ctx, req.UserID)
	s.Require().NoError(err)
	s.Empty(sessions)
}

func (s *SessionTestSuite) newSession(userID uuid.UUID) dto.NewSessionRequest {
	s.T().Helper()

	req := dto.NewSessionRequest{
		RequestTime: fixtureTime(), UserID: userID, SessionID: uuid.New(),
		RefreshToken: "original-refresh-token", Expiration: time.Minute, UA: "Test User Agent",
	}
	s.Require().NoError(s.valkeyRepo.NewSession(s.ctx, req))

	return req
}

//go:build integration

package valkey_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/dto"
)

func TestOAuthTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(OAuthTestSuite))
}

type OAuthTestSuite struct{ valkeyTestSuite }

func (s *OAuthTestSuite) TestOAuthStateRoundTrip() {
	userID := uuid.New()
	s.Require().NoError(s.valkeyRepo.NewOAuthState(s.ctx, dto.NewOAuthRequest{
		RequestTime: fixtureTime(), State: "known-state", UserID: userID, StateTTL: time.Minute,
	}))

	got, err := s.valkeyRepo.GetOAuthUserID(s.ctx, "known-state")
	s.Require().NoError(err)
	s.Equal(userID, got)
	_, err = s.valkeyRepo.GetOAuthUserID(s.ctx, "different-state")
	s.ErrorIs(err, valkey.Nil)
}

func (s *OAuthTestSuite) TestOAuthStateExpiration() {
	s.Require().NoError(s.valkeyRepo.NewOAuthState(s.ctx, dto.NewOAuthRequest{
		RequestTime: fixtureTime(), State: "expiring-state", UserID: uuid.New(), StateTTL: time.Second,
	}))
	s.Positive(s.ttl("oauthState:expiring-state"))
	s.Eventually(func() bool {
		_, err := s.valkeyRepo.GetOAuthUserID(s.ctx, "expiring-state")
		return errors.Is(err, valkey.Nil)
	}, 3*time.Second, 20*time.Millisecond)
}

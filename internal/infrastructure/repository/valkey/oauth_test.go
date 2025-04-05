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

func TestOAuthTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(OAuthTestSuite))
}

type OAuthTestSuite struct {
	suite.Suite
	ctx             context.Context
	valkeyContainer *containers.ValkeyContainer
	valkeyRepo      *valkeyRepo.Repository
}

func (s *OAuthTestSuite) SetupSuite() {
	s.ctx = context.Background()

	valkeyContainer, err := containers.NewValkeyContainer(s.ctx)
	s.Require().NoError(err)

	valkeyClient, err := valkey.NewClient(valkey.MustParseURL(valkeyContainer.ConnectionString))
	s.Require().NoError(err)

	repo := valkeyRepo.New(valkeyClient)

	s.valkeyContainer = valkeyContainer
	s.valkeyRepo = repo
}

func (s *OAuthTestSuite) TearDownSuite() {
	err := s.valkeyContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *OAuthTestSuite) TestNewOAuthUserState() {
	req := dto.NewOAuthRequest{
		RequestTime: time.Now().UTC(),
		State:       gofakeit.UUID(),
		UserID:      gofakeit.IntRange(1, 100),
		StateTTL:    2 * time.Second,
	}

	err := s.valkeyRepo.NewOAuthState(s.ctx, req)
	s.Require().NoError(err)

	_, err = s.valkeyRepo.GetOAuthUserID(s.ctx, req.State)
	s.Require().NoError(err)

	time.Sleep(3 * time.Second)

	_, err = s.valkeyRepo.GetOAuthUserID(s.ctx, req.State)
	s.Require().Error(err)
}

func (s *OAuthTestSuite) TestGetOAuthUserID() {
	req := dto.NewOAuthRequest{
		RequestTime: time.Now().UTC(),
		State:       gofakeit.UUID(),
		UserID:      gofakeit.IntRange(1, 100),
		StateTTL:    2 * time.Second,
	}

	err := s.valkeyRepo.NewOAuthState(s.ctx, req)
	s.Require().NoError(err)

	userID, err := s.valkeyRepo.GetOAuthUserID(s.ctx, req.State)
	s.Require().NoError(err)
	s.Require().Equal(req.UserID, userID)
}

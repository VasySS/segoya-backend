//go:build integration

package valkey_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/valkey-io/valkey-go"

	valkeyRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func fixtureTime() time.Time { return time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC) }

type valkeyTestSuite struct {
	suite.Suite

	ctx        context.Context
	client     valkey.Client
	valkeyRepo *valkeyRepo.Repository
}

func (s *valkeyTestSuite) SetupSuite() {
	s.client = containers.NewTestValkey(s.T())
}

func (s *valkeyTestSuite) SetupTest() {
	s.ctx = containers.TestContext(s.T())
	s.Require().NoError(s.client.Do(s.ctx, s.client.B().Flushdb().Build()).Error())
	s.valkeyRepo = valkeyRepo.New(s.ctx, s.client)
}

func (s *valkeyTestSuite) ttl(key string) int64 {
	s.T().Helper()
	ttl, err := s.client.Do(s.ctx, s.client.B().Pttl().Key(key).Build()).AsInt64()
	s.Require().NoError(err)

	return ttl
}

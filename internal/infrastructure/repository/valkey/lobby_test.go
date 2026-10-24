//go:build integration

package valkey_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

func TestLobbyTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(LobbyTestSuite))
}

type LobbyTestSuite struct{ valkeyTestSuite }

func (s *LobbyTestSuite) TestLobbyRoundTrip() {
	for _, tc := range []struct {
		name    string
		private bool
	}{
		{name: "public"}, {name: "private", private: true},
	} {
		s.Run(tc.name, func() {
			req := s.newLobby(tc.private)
			got, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
			s.Require().NoError(err)
			s.Equal(lobby.Lobby{
				ID: req.ID, CreatorID: req.CreatorID, CreatedAt: req.RequestTime,
				Provider: req.Provider, Rounds: req.Rounds, TimerSeconds: req.TimerSeconds,
				MaxPlayers: req.MaxPlayers, MovementAllowed: req.MovementAllowed, Private: tc.private,
			}, got)
		})
	}

	_, err := s.valkeyRepo.GetLobby(s.ctx, "missing")
	s.ErrorIs(err, lobby.ErrNotFound)
}

func (s *LobbyTestSuite) TestUpdateLobbyPreservesMembershipAndIdentity() {
	for _, tc := range []struct {
		name    string
		private bool
	}{
		{name: "public"}, {name: "private", private: true},
	} {
		s.Run(tc.name, func() {
			req := s.newLobby(tc.private)
			s.Require().NoError(s.valkeyRepo.AdmitLobbyUser(s.ctx, req.ID, req.CreatorID, "creator-session"))
			original, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
			s.Require().NoError(err)

			updated := original
			updated.Provider, updated.Rounds, updated.TimerSeconds, updated.MovementAllowed = "yandex", 5, 60, false
			s.Require().NoError(s.valkeyRepo.UpdateLobby(s.ctx, updated))
			got, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
			s.Require().NoError(err)
			s.Equal(updated, got)
			s.Require().NoError(s.valkeyRepo.AdmitLobbyUser(s.ctx, req.ID, req.CreatorID, "creator-session"))

			updated.CreatorID = uuid.New()
			s.Require().ErrorIs(s.valkeyRepo.UpdateLobby(s.ctx, updated), lobby.ErrOnlyCreatorCanUpdateSettings)
			got, err = s.valkeyRepo.GetLobby(s.ctx, req.ID)
			s.Require().NoError(err)
			s.Equal(original.CreatorID, got.CreatorID)
			s.Equal(1, got.CurrentPlayers)
		})
	}
}

func (s *LobbyTestSuite) TestGetLobbiesPagination() {
	lobbies, total, err := s.valkeyRepo.GetLobbies(s.ctx, dto.GetLobbiesRequest{Page: 1, PageSize: 2})
	s.Require().NoError(err)
	s.Empty(lobbies)
	s.Zero(total)

	// Distinct seconds make the sorted-set order explicit, independent of IDs.
	expected := []string{"public-3", "public-2", "public-1"}
	for i := 1; i <= 3; i++ {
		s.Require().NoError(s.valkeyRepo.NewLobby(s.ctx, dto.NewLobbyRequestDB{
			ID: expected[3-i], CreatorID: uuid.New(), RequestTime: fixtureTime().Add(time.Duration(i) * time.Second),
			Provider: "google", Rounds: 3, MaxPlayers: 2,
		}))
	}

	s.newLobby(true)

	for _, tc := range []struct {
		name string
		page int
		ids  []string
	}{
		{name: "first page", page: 1, ids: expected[:2]},
		{name: "partial last page", page: 2, ids: expected[2:]},
	} {
		s.Run(tc.name, func() {
			got, count, err := s.valkeyRepo.GetLobbies(s.ctx, dto.GetLobbiesRequest{Page: tc.page, PageSize: 2})
			s.Require().NoError(err)
			s.Equal(3, count)

			ids := make([]string, 0, len(got))
			for _, value := range got {
				ids = append(ids, value.ID)
				s.False(value.Private)
			}

			s.Equal(tc.ids, ids)
		})
	}
	// The existing repository returns zero total when the requested page is empty.
	// Keep that behavior separate from the total asserted on populated pages.
	empty, _, err := s.valkeyRepo.GetLobbies(s.ctx, dto.GetLobbiesRequest{Page: 3, PageSize: 2})
	s.Require().NoError(err)
	s.Empty(empty)
}

func (s *LobbyTestSuite) TestMembershipUsesMatchingSession() {
	req := s.newLobby(false)
	other := uuid.New()

	s.Require().NoError(s.valkeyRepo.AdmitLobbyUser(s.ctx, req.ID, req.CreatorID, "creator"))
	s.Require().NoError(s.valkeyRepo.AdmitLobbyUser(s.ctx, req.ID, other, "other"))
	s.Require().ErrorIs(s.valkeyRepo.AdmitLobbyUser(s.ctx, req.ID, uuid.New(), "extra"), lobby.ErrLobbyIsFull)
	s.Require().NoError(s.valkeyRepo.ReleaseLobbyUser(s.ctx, req.ID, req.CreatorID, "stale", time.Minute))
	got, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(2, got.CurrentPlayers)
	s.Require().NoError(s.valkeyRepo.ReleaseLobbyUser(s.ctx, req.ID, req.CreatorID, "creator", time.Minute))
	s.Require().NoError(s.valkeyRepo.ReleaseLobbyUser(s.ctx, req.ID, req.CreatorID, "creator", time.Minute))
	got, err = s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(1, got.CurrentPlayers)
}

func (s *LobbyTestSuite) TestDeleteLobbyRemovesPublicIndex() {
	for _, tc := range []struct {
		name    string
		private bool
	}{
		{name: "public"}, {name: "private", private: true},
	} {
		s.Run(tc.name, func() {
			req := s.newLobby(tc.private)
			s.Require().NoError(s.valkeyRepo.DeleteLobby(s.ctx, req.ID))
			_, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
			s.ErrorIs(err, lobby.ErrNotFound)
		})
	}

	got, total, err := s.valkeyRepo.GetLobbies(s.ctx, dto.GetLobbiesRequest{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Empty(got)
	s.Zero(total)
}

func (s *LobbyTestSuite) TestInitialLobbyExpiration() {
	req := dto.NewLobbyRequestDB{
		ID: uuid.NewString(), CreatorID: uuid.New(), RequestTime: fixtureTime(),
		Provider: "google", Rounds: 3, MaxPlayers: 2, Expiration: time.Second,
	}
	s.Require().NoError(s.valkeyRepo.NewLobby(s.ctx, req))
	s.Positive(s.ttl("lobby:" + req.ID))
	s.Eventually(func() bool {
		_, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
		return errors.Is(err, lobby.ErrNotFound)
	}, 3*time.Second, 20*time.Millisecond)
}

func (s *LobbyTestSuite) TestAdmissionAndLastReleaseChangeExpiration() {
	req := s.newLobby(false)
	key := "lobby:" + req.ID
	s.Positive(s.ttl(key))
	s.Require().NoError(s.valkeyRepo.AdmitLobbyUser(s.ctx, req.ID, req.CreatorID, "creator"))
	s.EqualValues(-1, s.ttl(key))
	s.Require().NoError(s.valkeyRepo.ReleaseLobbyUser(s.ctx, req.ID, req.CreatorID, "creator", time.Minute))
	s.Positive(s.ttl(key))
	s.LessOrEqual(s.ttl(key), int64(time.Minute/time.Millisecond))
}

func (s *LobbyTestSuite) newLobby(private bool) dto.NewLobbyRequestDB {
	s.T().Helper()

	req := dto.NewLobbyRequestDB{
		ID: uuid.NewString(), CreatorID: uuid.New(), RequestTime: fixtureTime(),
		Provider: "google", Rounds: 3, TimerSeconds: 30, MaxPlayers: 2,
		MovementAllowed: true, Expiration: time.Minute,
	}
	if private {
		s.Require().NoError(s.valkeyRepo.NewPrivateLobby(s.ctx, req))
	} else {
		s.Require().NoError(s.valkeyRepo.NewLobby(s.ctx, req))
	}

	return req
}

package valkey_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/suite"
	"github.com/valkey-io/valkey-go"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	valkeyRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func TestLobbyTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LobbyTestSuite))
}

type LobbyTestSuite struct {
	suite.Suite
	ctx             context.Context
	valkeyContainer *containers.ValkeyContainer
	valkeyRepo      *valkeyRepo.Repository
}

func (s *LobbyTestSuite) SetupSuite() {
	s.ctx = context.Background()

	valkeyContainer, err := containers.NewValkeyContainer(s.ctx)
	s.Require().NoError(err)

	valkeyClient, err := valkey.NewClient(valkey.MustParseURL(valkeyContainer.ConnectionString))
	s.Require().NoError(err)

	repo := valkeyRepo.New(s.ctx, valkeyClient)

	s.valkeyContainer = valkeyContainer
	s.valkeyRepo = repo
}

func (s *LobbyTestSuite) TearDownSuite() {
	err := s.valkeyContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *LobbyTestSuite) TestNewLobby() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	l, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(req.ID, l.ID)
	s.Equal(req.CreatorID, l.CreatorID)
	s.WithinDuration(req.RequestTime, l.CreatedAt, 1*time.Second)
	s.Equal(req.Rounds, l.Rounds)
	s.Equal(req.Provider, l.Provider)
	s.Equal(req.TimerSeconds, l.TimerSeconds)
	s.Equal(req.MovementAllowed, l.MovementAllowed)
	s.Equal(req.MaxPlayers, l.MaxPlayers)
	s.Equal(0, l.CurrentPlayers)
	s.False(l.Private)
}

func (s *LobbyTestSuite) TestNewPrivateLobby() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "yandex",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: false,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewPrivateLobby(s.ctx, req)
	s.Require().NoError(err)

	l, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(req.ID, l.ID)
	s.Equal(req.CreatorID, l.CreatorID)
	s.WithinDuration(req.RequestTime, l.CreatedAt, 1*time.Second)
	s.Equal(req.Rounds, l.Rounds)
	s.Equal(req.Provider, l.Provider)
	s.Equal(req.TimerSeconds, l.TimerSeconds)
	s.Equal(req.MovementAllowed, l.MovementAllowed)
	s.Equal(req.MaxPlayers, l.MaxPlayers)
	s.Equal(0, l.CurrentPlayers)
	s.True(l.Private)
}

func (s *LobbyTestSuite) TestGetLobby() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	l, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)

	s.Equal(req.ID, l.ID)
	s.Equal(req.CreatorID, l.CreatorID)
	s.Equal(req.Rounds, l.Rounds)
	s.Equal(req.Provider, l.Provider)
	s.Equal(req.TimerSeconds, l.TimerSeconds)
	s.Equal(req.MovementAllowed, l.MovementAllowed)
	s.Equal(req.MaxPlayers, l.MaxPlayers)
	s.Equal(0, l.CurrentPlayers)
	s.WithinDuration(req.RequestTime, l.CreatedAt, 1*time.Second)

	_, err = s.valkeyRepo.GetLobby(s.ctx, "nonexistent")
	s.Require().Error(err)
	s.Equal(lobby.ErrNotFound, err)
}

func (s *LobbyTestSuite) TestUpdateLobby() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	oldLobby, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)

	newProvider := gofakeit.LetterN(5)
	newRounds := gofakeit.IntRange(1, 10)
	newTimerSeconds := gofakeit.IntRange(10, 60)
	newMovementAllowed := gofakeit.Bool()

	err = s.valkeyRepo.UpdateLobby(s.ctx, lobby.Lobby{
		ID:              oldLobby.ID,
		CreatorID:       oldLobby.CreatorID,
		CreatedAt:       oldLobby.CreatedAt,
		Rounds:          newRounds,
		Provider:        newProvider,
		TimerSeconds:    newTimerSeconds,
		MovementAllowed: newMovementAllowed,
		MaxPlayers:      oldLobby.MaxPlayers,
		CurrentPlayers:  oldLobby.CurrentPlayers,
	})
	s.Require().NoError(err)

	updatedLobby, err := s.valkeyRepo.GetLobby(s.ctx, oldLobby.ID)
	s.Require().NoError(err)
	s.Equal(oldLobby.ID, updatedLobby.ID)
	s.Equal(oldLobby.CreatorID, updatedLobby.CreatorID)
	s.Equal(oldLobby.CreatedAt, updatedLobby.CreatedAt)
	s.Equal(oldLobby.MaxPlayers, updatedLobby.MaxPlayers)
	s.Equal(oldLobby.CurrentPlayers, updatedLobby.CurrentPlayers)

	s.Equal(newRounds, updatedLobby.Rounds)
	s.Equal(newProvider, updatedLobby.Provider)
	s.Equal(newTimerSeconds, updatedLobby.TimerSeconds)
	s.Equal(newMovementAllowed, updatedLobby.MovementAllowed)
}

func (s *LobbyTestSuite) TestGetLobbies() {
	newLobbyIDs := make([]string, 0, 3)

	for range 3 {
		req := dto.NewLobbyRequestDB{
			ID:              gofakeit.UUID(),
			CreatorID:       gofakeit.IntRange(1, 100),
			RequestTime:     time.Now().UTC(),
			Rounds:          gofakeit.IntRange(1, 10),
			Provider:        "google",
			TimerSeconds:    gofakeit.IntRange(10, 60),
			MovementAllowed: true,
			MaxPlayers:      gofakeit.IntRange(2, 10),
		}

		err := s.valkeyRepo.NewLobby(s.ctx, req)
		s.Require().NoError(err)

		newLobbyIDs = append(newLobbyIDs, req.ID)
	}

	lobbies, total, err := s.valkeyRepo.GetLobbies(s.ctx, dto.GetLobbiesRequest{
		Page:     1,
		PageSize: 3,
	})
	s.Require().NoError(err)
	s.Require().Len(lobbies, 3)
	s.GreaterOrEqual(total, 3)

	gotLobbyIDs := make([]string, len(lobbies))
	for i, l := range lobbies {
		gotLobbyIDs[i] = l.ID

		// all lobbies should be public in pagination
		s.False(l.Private)
	}

	for _, id := range newLobbyIDs {
		s.Contains(gotLobbyIDs, id)
	}
}

func (s *LobbyTestSuite) TestIncrementLobbyPlayers() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	for range 2 {
		err = s.valkeyRepo.IncrementLobbyPlayers(s.ctx, req.ID)
		s.Require().NoError(err)
	}

	l, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(2, l.CurrentPlayers)
}

func (s *LobbyTestSuite) TestDecrementLobbyPlayers() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	for range 2 {
		err = s.valkeyRepo.IncrementLobbyPlayers(s.ctx, req.ID)
		s.Require().NoError(err)
	}

	err = s.valkeyRepo.DecrementLobbyPlayers(s.ctx, req.ID)
	s.Require().NoError(err)

	l, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(1, l.CurrentPlayers)
}

func (s *LobbyTestSuite) TestDeleteLobby() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	l, err := s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(req.ID, l.ID)

	err = s.valkeyRepo.DeleteLobby(s.ctx, req.ID)
	s.Require().NoError(err)

	_, err = s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Equal(lobby.ErrNotFound, err)
}

func (s *LobbyTestSuite) TestAddLobbyExpiration() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	err = s.valkeyRepo.AddLobbyExpiration(s.ctx, req.ID, 2*time.Second)
	s.Require().NoError(err)

	_, err = s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)

	time.Sleep(3 * time.Second)

	_, err = s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Equal(lobby.ErrNotFound, err)
}

func (s *LobbyTestSuite) TestDeleteLobbyExpiration() {
	req := dto.NewLobbyRequestDB{
		ID:              gofakeit.UUID(),
		CreatorID:       gofakeit.IntRange(1, 100),
		RequestTime:     time.Now().UTC(),
		Rounds:          gofakeit.IntRange(1, 10),
		Provider:        "google",
		TimerSeconds:    gofakeit.IntRange(10, 60),
		MovementAllowed: true,
		MaxPlayers:      gofakeit.IntRange(2, 10),
	}

	err := s.valkeyRepo.NewLobby(s.ctx, req)
	s.Require().NoError(err)

	err = s.valkeyRepo.AddLobbyExpiration(s.ctx, req.ID, 2*time.Second)
	s.Require().NoError(err)

	err = s.valkeyRepo.DeleteLobbyExpiration(s.ctx, req.ID)
	s.Require().NoError(err)

	time.Sleep(3 * time.Second)

	_, err = s.valkeyRepo.GetLobby(s.ctx, req.ID)
	s.Require().NoError(err)
}

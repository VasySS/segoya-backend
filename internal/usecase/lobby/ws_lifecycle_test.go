package lobby_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	lobbyEntity "github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby/mocks"
)

func TestUsecase_LobbyUserConnect(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"success", "profile lookup failure", "admission failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			users, repo := mocks.NewUserRepository(t), mocks.NewRepository(t)
			id := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")
			profile := user.PrivateProfile{PublicProfile: user.PublicProfile{ID: id, Username: "alice"}, Password: "secret"}
			cause := errors.New("lobby unavailable")

			var profileErr, admissionErr error

			if scenario == "profile lookup failure" {
				profileErr = cause
			}

			if scenario == "admission failure" {
				admissionErr = cause
			}

			lookup := users.On("GetUserByID", mock.Anything, id).Return(profile, profileErr).Once()

			if profileErr == nil {
				admit := repo.On("AdmitLobbyUser", mock.Anything, "lobby", id, "session").Return(admissionErr).Once()
				mock.InOrder(lookup, admit)
			}

			uc := lobby.NewUsecase(lobby.Config{}, nil, users, repo, nil)

			got, err := uc.ConnectLobbyUser(t.Context(), "lobby", id, "session")
			if scenario != "success" {
				require.ErrorIs(t, err, cause)
				assert.Equal(t, user.PublicProfile{}, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, profile.ToPublicProfile(), got)
		})
	}
}

func TestUsecase_LobbyUserDisconnect(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"success", "release failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			repo := mocks.NewRepository(t)
			id := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")

			var want error
			if scenario != "success" {
				want = errors.New("release unavailable")
			}

			repo.On("ReleaseLobbyUser", mock.Anything, "lobby", id, "session", 3*time.Minute).Return(want).Once()
			uc := lobby.NewUsecase(lobby.Config{LobbyExpiration: 3 * time.Minute}, nil, nil, repo, nil)
			require.ErrorIs(t, uc.DisconnectLobbyUser(t.Context(), "lobby", id, "session"), want)
		})
	}
}

func TestUsecase_LobbyGameStart(t *testing.T) {
	t.Parallel()

	cause := errors.New("dependency unavailable")
	for _, tt := range []struct {
		name                                string
		lookupErr, gameErr, deleteErr, want error
		change                              func(*lobbyEntity.Lobby)
		create, remove                      bool
	}{
		{name: "success", create: true, remove: true},
		{name: "lookup failure", lookupErr: cause, want: cause},
		{
			name: "not creator", want: lobbyEntity.ErrOnlyCreatorCanStart,
			change: func(l *lobbyEntity.Lobby) { l.CreatorID = uuid.Nil },
		},
		{
			name: "too few players", want: lobbyEntity.ErrInvalidPlayers,
			change: func(l *lobbyEntity.Lobby) { l.CurrentPlayers = 1 },
		},
		{
			name: "invalid settings", want: game.ErrInvalidRounds,
			change: func(l *lobbyEntity.Lobby) { l.Rounds = 0 },
		},
		{name: "game creation failure", gameErr: cause, want: cause, create: true},
		{name: "delete failure", deleteErr: cause, want: cause, create: true, remove: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, games := mocks.NewRepository(t), mocks.NewMultiplayerUsecase(t)
			creator := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")
			second := uuid.MustParse("019a553a-fc00-7000-8000-000000000002")
			gameID := uuid.MustParse("019a553a-fc00-7000-8000-000000000003")
			req := dto.StartLobbyGameRequest{
				RequestTime: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC), LobbyID: "lobby",
				Creator: user.PublicProfile{ID: creator}, ConnectedPlayers: []user.PublicProfile{{ID: creator}, {ID: second}},
			}

			stored := lobbyEntity.Lobby{
				ID: req.LobbyID, CreatorID: creator, CurrentPlayers: 2, MaxPlayers: 5,
				Provider: "google", Rounds: 10, TimerSeconds: 60, MovementAllowed: true,
			}
			if tt.change != nil {
				tt.change(&stored)
			}

			lookup := repo.On("GetLobby", mock.Anything, req.LobbyID).Return(stored, tt.lookupErr).Once()

			if tt.create {
				start := games.On("NewGame", mock.Anything, dto.NewMultiplayerGameRequest{
					RequestTime: req.RequestTime, CreatorID: creator, ConnectedPlayers: req.ConnectedPlayers,
					Rounds: 10, TimerSeconds: 60, Provider: "google", MovementAllowed: true,
				}).Return(gameID, tt.gameErr).Once()
				mock.InOrder(lookup, start)

				if tt.remove {
					remove := repo.On("DeleteLobby", mock.Anything, req.LobbyID).Return(tt.deleteErr).Once()
					mock.InOrder(start, remove)
				}
			}

			uc := lobby.NewUsecase(lobby.Config{}, nil, nil, repo, games)
			got, err := uc.StartLobbyGame(t.Context(), req)
			require.ErrorIs(t, err, tt.want)

			if tt.want != nil {
				assert.Equal(t, uuid.Nil, got)
			} else {
				assert.Equal(t, gameID, got)
			}

			if !tt.remove {
				repo.AssertNotCalled(t, "DeleteLobby", mock.Anything, mock.Anything)
			}
		})
	}
}

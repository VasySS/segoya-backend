package lobby_test

import (
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	lobbyEntity "github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUsecase_LobbyUserConnect(t *testing.T) {
	t.Parallel()

	type fields struct {
		lobbyRepo *mocks.Repository
		userRepo  *mocks.UserRepository
	}

	type args struct {
		lobbyID string
		userID  int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    user.PublicProfile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully connect user to lobby",
			args: args{
				lobbyID: "1234567890",
				userID:  1,
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.lobbyID).
					Return(lobbyEntity.Lobby{
						ID:             args.lobbyID,
						CurrentPlayers: 4,
						MaxPlayers:     5,
					}, nil)
				fs.lobbyRepo.On("IncrementLobbyPlayers", mock.Anything, args.lobbyID).
					Return(nil)
				fs.lobbyRepo.On("DeleteLobbyExpiration", mock.Anything, args.lobbyID).
					Return(nil)
				fs.userRepo.On("GetUserByID", mock.Anything, args.userID).
					Return(user.PrivateProfile{PublicProfile: user.PublicProfile{ID: args.userID}}, nil)
			},
			want:    user.PublicProfile{ID: 1},
			wantErr: assert.NoError,
		},
		{
			name: "trying to connect to full lobby",
			args: args{
				lobbyID: "1234567890",
				userID:  1,
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.lobbyID).
					Return(lobbyEntity.Lobby{
						ID:             args.lobbyID,
						CurrentPlayers: 5,
						MaxPlayers:     5,
					}, nil)
			},
			want:    user.PublicProfile{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			userRepo := mocks.NewUserRepository(t)
			fs := fields{
				lobbyRepo: lobbyRepo,
				userRepo:  userRepo,
			}
			tt.setup(fs, tt.args)

			uc := lobby.NewUsecase(lobby.Config{}, nil, userRepo, lobbyRepo, nil)

			got, err := uc.ConnectLobbyUser(t.Context(), tt.args.lobbyID, tt.args.userID)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_LobbyUserDisconnect(t *testing.T) {
	t.Parallel()

	type fields struct {
		conf      lobby.Config
		lobbyRepo *mocks.Repository
	}

	type args struct {
		lobbyID string
		_       int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully disconnect user from lobby (last user)",
			args: args{
				lobbyID: "1234567890",
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.lobbyID).
					Return(lobbyEntity.Lobby{
						ID:             args.lobbyID,
						CurrentPlayers: 1,
						MaxPlayers:     5,
					}, nil)

				fs.lobbyRepo.On("AddLobbyExpiration", mock.Anything, args.lobbyID, fs.conf.LobbyExpiration).
					Return(nil)

				fs.lobbyRepo.On("DecrementLobbyPlayers", mock.Anything, args.lobbyID).
					Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "successfully disconnect user from lobby (not last user)",
			args: args{
				lobbyID: "1234567890",
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.lobbyID).
					Return(lobbyEntity.Lobby{
						ID:             args.lobbyID,
						CurrentPlayers: 2,
						MaxPlayers:     5,
					}, nil)

				fs.lobbyRepo.On("DecrementLobbyPlayers", mock.Anything, args.lobbyID).
					Return(nil)
			},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			conf := lobby.Config{
				LobbyExpiration: 3 * time.Minute,
			}
			fs := fields{
				conf:      conf,
				lobbyRepo: lobbyRepo,
			}
			tt.setup(fs, tt.args)

			uc := lobby.NewUsecase(conf, nil, nil, lobbyRepo, nil)

			err := uc.DisconnectLobbyUser(t.Context(), tt.args.lobbyID, 0)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_LobbyGameStart(t *testing.T) {
	t.Parallel()

	startLobbyReq := dto.StartLobbyGameRequest{
		RequestTime: time.Now().UTC(),
		LobbyID:     "1234567890",
		Creator: user.PublicProfile{
			ID: 1,
		},
		ConnectedPlayers: []user.PublicProfile{
			{
				ID: 1,
			},
			{
				ID: 3,
			},
		},
	}

	type fields struct {
		lobbyRepo *mocks.Repository
		mult      *mocks.MultiplayerUsecase
	}

	type args struct {
		req dto.StartLobbyGameRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully start game",
			args: args{
				req: startLobbyReq,
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.req.LobbyID).
					Return(lobbyEntity.Lobby{
						ID:              args.req.LobbyID,
						CreatorID:       args.req.Creator.ID,
						CurrentPlayers:  2,
						MaxPlayers:      5,
						Rounds:          10,
						TimerSeconds:    60,
						MovementAllowed: true,
						Provider:        "google",
					}, nil)

				fs.mult.On("NewGame", mock.Anything, dto.NewMultiplayerGameRequest{
					RequestTime:      args.req.RequestTime,
					CreatorID:        args.req.Creator.ID,
					ConnectedPlayers: args.req.ConnectedPlayers,
					Rounds:           10,
					TimerSeconds:     60,
					Provider:         "google",
					MovementAllowed:  true,
				}).Return(1, nil)

				fs.lobbyRepo.On("DeleteLobby", mock.Anything, args.req.LobbyID).
					Return(nil)
			},
			want:    1,
			wantErr: assert.NoError,
		},
		{
			name: "trying to start game as not creator",
			args: args{
				req: startLobbyReq,
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.req.LobbyID).
					Return(lobbyEntity.Lobby{
						ID:              args.req.LobbyID,
						CreatorID:       777,
						CurrentPlayers:  2,
						MaxPlayers:      5,
						Rounds:          10,
						TimerSeconds:    60,
						MovementAllowed: true,
						Provider:        "google",
					}, nil)
			},
			want:    0,
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			mult := mocks.NewMultiplayerUsecase(t)
			fs := fields{
				lobbyRepo: lobbyRepo,
				mult:      mult,
			}
			tt.setup(fs, tt.args)

			uc := lobby.NewUsecase(lobby.Config{}, nil, nil, lobbyRepo, mult)

			gameID, err := uc.StartLobbyGame(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, gameID)
		})
	}
}

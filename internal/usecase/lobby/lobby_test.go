package lobby_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/VasySS/segoya-backend/internal/dto"
	lobbyEntity "github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby/mocks"
)

func TestUsecase_NewLobby(t *testing.T) {
	t.Parallel()

	firstUserID := uuid.Must(uuid.NewV7())
	secondUserID := uuid.Must(uuid.NewV7())

	newPublicLobbyReq := dto.NewLobbyRequest{
		RequestTime:     time.Now().UTC(),
		MaxPlayers:      10,
		CreatorID:       firstUserID,
		Rounds:          10,
		Provider:        "google",
		TimerSeconds:    30,
		MovementAllowed: true,
		Private:         false,
	}

	newPrivateLobbyReq := dto.NewLobbyRequest{
		RequestTime:     time.Now().UTC(),
		MaxPlayers:      10,
		CreatorID:       secondUserID,
		Rounds:          10,
		Provider:        "yandex",
		TimerSeconds:    60,
		MovementAllowed: true,
		Private:         true,
	}

	type fields struct {
		conf      lobby.Config
		rnd       *mocks.RandomGenerator
		lobbyRepo *mocks.Repository
	}

	type args struct {
		req dto.NewLobbyRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully create public lobby",
			args: args{
				req: newPublicLobbyReq,
			},
			setup: func(fs fields, args args) {
				lobbyID := "1234567890"

				fs.rnd.On("NewRandomHexString", fs.conf.LobbyIDLength).
					Return(lobbyID)

				fs.lobbyRepo.On("NewLobby", mock.Anything, dto.NewLobbyRequestDB{
					ID:              lobbyID,
					CreatorID:       args.req.CreatorID,
					RequestTime:     args.req.RequestTime,
					Rounds:          args.req.Rounds,
					MaxPlayers:      args.req.MaxPlayers,
					Provider:        args.req.Provider,
					TimerSeconds:    args.req.TimerSeconds,
					MovementAllowed: args.req.MovementAllowed,
				}).Return(nil)

				fs.lobbyRepo.On("AddLobbyExpiration", mock.Anything, lobbyID, mock.AnythingOfType("time.Duration")).
					Return(nil)
			},
			want:    "1234567890",
			wantErr: assert.NoError,
		},
		{
			name: "successfully create private lobby",
			args: args{
				req: newPrivateLobbyReq,
			},
			setup: func(fs fields, args args) {
				lobbyID := "112233"

				fs.rnd.On("NewRandomHexString", fs.conf.LobbyIDLength).
					Return(lobbyID)

				fs.lobbyRepo.On("NewPrivateLobby", mock.Anything, dto.NewLobbyRequestDB{
					ID:              lobbyID,
					CreatorID:       args.req.CreatorID,
					RequestTime:     args.req.RequestTime,
					Rounds:          args.req.Rounds,
					MaxPlayers:      args.req.MaxPlayers,
					Provider:        args.req.Provider,
					TimerSeconds:    args.req.TimerSeconds,
					MovementAllowed: args.req.MovementAllowed,
				}).Return(nil)

				fs.lobbyRepo.On("AddLobbyExpiration", mock.Anything, lobbyID, mock.AnythingOfType("time.Duration")).
					Return(nil)
			},
			want:    "112233",
			wantErr: assert.NoError,
		},
		{
			name: "failed to create lobby",
			args: args{
				req: newPublicLobbyReq,
			},
			setup: func(fs fields, _ args) {
				fs.rnd.On("NewRandomHexString", fs.conf.LobbyIDLength).
					Return("1234567890")

				fs.lobbyRepo.On("NewLobby", mock.Anything, mock.Anything).
					Return(errors.New("some db error"))
			},
			want:    "",
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			rnd := mocks.NewRandomGenerator(t)
			conf := lobby.Config{LobbyIDLength: 10}
			fs := fields{
				conf:      conf,
				rnd:       rnd,
				lobbyRepo: lobbyRepo,
			}
			tt.setup(fs, tt.args)

			uc := lobby.NewUsecase(conf, rnd, nil, lobbyRepo, nil)

			got, err := uc.NewLobby(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetLobby(t *testing.T) {
	t.Parallel()

	type fields struct {
		lobbyRepo *mocks.Repository
	}

	type args struct {
		id string
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    lobbyEntity.Lobby
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get lobby",
			args: args{
				id: "1234567890",
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.id).
					Return(lobbyEntity.Lobby{ID: args.id}, nil)
			},
			want:    lobbyEntity.Lobby{ID: "1234567890"},
			wantErr: assert.NoError,
		},
		{
			name: "failed to get lobby",
			args: args{
				id: "1234567890",
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.id).
					Return(lobbyEntity.Lobby{}, errors.New("some db error"))
			},
			want:    lobbyEntity.Lobby{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			fs := fields{
				lobbyRepo: lobbyRepo,
			}
			tt.setup(fs, tt.args)

			uc := lobby.NewUsecase(lobby.Config{}, nil, nil, lobbyRepo, nil)

			got, err := uc.GetLobby(t.Context(), tt.args.id)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetLobbies(t *testing.T) {
	t.Parallel()

	firstUserID := uuid.Must(uuid.NewV7())
	secondUserID := uuid.Must(uuid.NewV7())

	getLobbiesResponse := []lobbyEntity.Lobby{
		{
			ID:        "1234567890",
			CreatorID: firstUserID,
		},
		{
			ID:        "1234567891",
			CreatorID: secondUserID,
		},
	}

	type args struct {
		req dto.GetLobbiesRequest
	}

	type fields struct {
		lobbyRepo *mocks.Repository
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []lobbyEntity.Lobby
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get lobbies",
			args: args{
				req: dto.GetLobbiesRequest{
					Page:     1,
					PageSize: 10,
				},
			},
			setup: func(fs fields, a args) {
				fs.lobbyRepo.On("GetLobbies", mock.Anything, a.req).
					Return(getLobbiesResponse, len(getLobbiesResponse), nil)
			},
			want:    getLobbiesResponse,
			wantErr: assert.NoError,
		},
		{
			name: "failed to get lobbies",
			args: args{
				req: dto.GetLobbiesRequest{
					Page:     1,
					PageSize: 10,
				},
			},
			setup: func(fs fields, a args) {
				fs.lobbyRepo.On("GetLobbies", mock.Anything, a.req).
					Return([]lobbyEntity.Lobby{}, len(getLobbiesResponse), errors.New("some db error"))
			},
			want:    []lobbyEntity.Lobby(nil),
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			fs := fields{
				lobbyRepo: lobbyRepo,
			}
			tt.setup(fs, tt.args)

			uc := lobby.NewUsecase(lobby.Config{}, nil, nil, lobbyRepo, nil)

			got, total, err := uc.GetLobbies(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Len(t, got, total)
			assert.Equal(t, tt.want, got)
		})
	}
}

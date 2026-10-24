package lobby_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/VasySS/segoya-backend/internal/dto"
	lobbyEntity "github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby/mocks"
)

func TestUsecase_UpdateLobbySettings(t *testing.T) {
	t.Parallel()

	type args struct {
		req dto.UpdateLobbySettingsRequest
	}

	type fields struct {
		lobbyRepo *mocks.Repository
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully update lobby settings",
			args: args{
				req: dto.UpdateLobbySettingsRequest{
					RequestTime: time.Now().UTC(),
					LobbyID:     "1234567890",
					Creator: user.PublicProfile{
						ID: uuid.Must(uuid.NewV7()),
					},
					Settings: dto.LobbySettingsMessage{
						Provider:        "google",
						MovementAllowed: true,
						Rounds:          10,
						TimerSeconds:    60,
					},
				},
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.req.LobbyID).
					Return(lobbyEntity.Lobby{
						ID:              args.req.LobbyID,
						CreatorID:       args.req.Creator.ID,
						CurrentPlayers:  2,
						MaxPlayers:      5,
						Rounds:          5,
						TimerSeconds:    30,
						MovementAllowed: false,
						Provider:        "yandex",
					}, nil)

				fs.lobbyRepo.On("UpdateLobby", mock.Anything, lobbyEntity.Lobby{
					ID:              args.req.LobbyID,
					CreatorID:       args.req.Creator.ID,
					CurrentPlayers:  2,
					MaxPlayers:      5,
					Rounds:          10,
					TimerSeconds:    60,
					MovementAllowed: true,
					Provider:        "google",
				}).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "trying to update settings as not creator",
			args: args{
				req: dto.UpdateLobbySettingsRequest{
					RequestTime: time.Now().UTC(),
					LobbyID:     "1234567890",
					Creator: user.PublicProfile{
						ID: uuid.Must(uuid.NewV7()),
					},
					Settings: dto.LobbySettingsMessage{
						Provider:        "google",
						MovementAllowed: true,
						Rounds:          10,
						TimerSeconds:    60,
					},
				},
			},
			setup: func(fs fields, args args) {
				fs.lobbyRepo.On("GetLobby", mock.Anything, args.req.LobbyID).
					Return(lobbyEntity.Lobby{
						ID:              args.req.LobbyID,
						CreatorID:       uuid.Must(uuid.NewV7()),
						CurrentPlayers:  2,
						MaxPlayers:      5,
						Rounds:          5,
						TimerSeconds:    30,
						MovementAllowed: false,
						Provider:        "yandex",
					}, nil)
			},
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

			err := uc.UpdateLobbySettings(t.Context(), tt.args.req)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_UpdateLobbySettingsPreservesRepositoryContract(t *testing.T) {
	t.Parallel()

	creatorID := uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6943")
	original := lobbyEntity.Lobby{
		ID: "test-lobby", CreatorID: creatorID, CreatedAt: time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC),
		Provider: "google", MovementAllowed: true, Rounds: 5, TimerSeconds: 60,
		CurrentPlayers: 2, MaxPlayers: 5, Private: true,
	}
	tests := []struct {
		name     string
		settings dto.LobbySettingsMessage
		want     lobbyEntity.Lobby
		wantErr  error
	}{
		{
			name:     "unchanged settings still persist",
			settings: dto.LobbySettingsMessage{Provider: "google", MovementAllowed: true, Rounds: 5, TimerSeconds: 60},
			want:     original,
		},
		{
			name:     "invalid settings rejected by usecase",
			wantErr:  lobbyEntity.ErrInvalidProvider,
			settings: dto.LobbySettingsMessage{Provider: "unknown", Rounds: 0, TimerSeconds: 9},
			want: lobbyEntity.Lobby{
				ID: "test-lobby", CreatorID: creatorID, CreatedAt: original.CreatedAt,
				Provider: "unknown", Rounds: 0, TimerSeconds: 9,
				CurrentPlayers: 2, MaxPlayers: 5, Private: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lobbyRepo := mocks.NewRepository(t)
			lobbyRepo.On("GetLobby", mock.Anything, original.ID).Return(original, nil).Once()

			if tt.wantErr == nil {
				lobbyRepo.On("UpdateLobby", mock.Anything, tt.want).Return(nil).Once()
			}

			uc := lobby.NewUsecase(lobby.Config{}, nil, nil, lobbyRepo, nil)

			err := uc.UpdateLobbySettings(t.Context(), dto.UpdateLobbySettingsRequest{
				LobbyID: original.ID, Creator: user.PublicProfile{ID: creatorID}, Settings: tt.settings,
			})
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

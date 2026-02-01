package multiplayer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	multiplayerEntity "github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer/mocks"
)

func TestUsecase_NewMultiplayerRoundGuess(t *testing.T) {
	t.Parallel()

	saveGuessReq := dto.NewMultiplayerRoundGuessRequest{
		RequestTime: time.Now().UTC(),
		UserID:      uuid.Must(uuid.NewV7()),
		GameID:      uuid.Must(uuid.NewV7()),
		Guess: game.LatLng{
			Lat: 1.0,
			Lng: 2.0,
		},
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.NewMultiplayerRoundGuessRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully save user guess",
			args: args{
				req: saveGuessReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				gameResponse := multiplayerEntity.Game{
					ID:           args.req.GameID,
					RoundCurrent: 2,
					Provider:     "google",
				}

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(gameResponse, nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).
					Return([]user.MultiplayerUser{
						{
							PublicProfile: user.PublicProfile{ID: saveGuessReq.UserID, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"},
							Connected:     true,
						},
					}, nil)

				roundResponse := multiplayerEntity.Round{
					ID:       uuid.Must(uuid.NewV7()),
					RoundNum: gameResponse.RoundCurrent,
					Finished: false,
				}

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, gameResponse.RoundCurrent).
					Return(roundResponse, nil)

				fs.pano.On("CalculateScoreAndDistance",
					gameResponse.Provider, roundResponse.Lat, roundResponse.Lng, args.req.Guess.Lat, args.req.Guess.Lng).
					Return(4567, 1234)

				fs.repo.On("NewMultiplayerRoundGuess", mock.Anything, dto.NewMultiplayerRoundGuessRequestDB{
					RequestTime: args.req.RequestTime,
					RoundID:     roundResponse.ID,
					UserID:      args.req.UserID,
					Lat:         args.req.Guess.Lat,
					Lng:         args.req.Guess.Lng,
					Score:       4567,
					Distance:    1234,
				}).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "trying to save guess for finished round",
			args: args{
				req: saveGuessReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				gameResponse := multiplayerEntity.Game{
					ID:           args.req.GameID,
					RoundCurrent: 2,
					Provider:     "google",
				}

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(gameResponse, nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).
					Return([]user.MultiplayerUser{
						{
							PublicProfile: user.PublicProfile{ID: saveGuessReq.UserID, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"},
							Connected:     true,
						},
					}, nil)

				roundResponse := multiplayerEntity.Round{
					ID:       uuid.Must(uuid.NewV7()),
					RoundNum: gameResponse.RoundCurrent,
					Finished: true,
				}

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, gameResponse.RoundCurrent).
					Return(roundResponse, nil)
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, multiplayerEntity.ErrRoundAlreadyFinished)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			fs := fields{
				repo: repo,
				pano: pano,
			}
			tt.setup(fs, tt.args)

			uc := multiplayer.NewUsecase(multiplayer.Config{}, repo, pano)

			err := uc.NewRoundGuess(t.Context(), tt.args.req)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_EndRound(t *testing.T) {
	t.Parallel()

	endRoundRequest := dto.EndMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      uuid.Must(uuid.NewV7()),
		UserID:      uuid.Must(uuid.NewV7()),
	}

	endRoundResponse := []multiplayerEntity.Guess{
		{
			Username: "username1",
			RoundNum: 1,
			Score:    4567,
		},
		{
			Username: "username2",
			RoundNum: 1,
			Score:    1234,
		},
	}

	mockUsers := []user.MultiplayerUser{
		{PublicProfile: user.PublicProfile{ID: endRoundRequest.UserID, Username: "username1"}, Connected: true},
		{PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"}, Connected: true},
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.EndMultiplayerRoundRequest
	}

	setupCommonMocks := func(fs fields, args args, round multiplayerEntity.Round, game multiplayerEntity.Game) {
		fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
			Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) })

		fs.repo.On("LockMultiplayerGame", mock.Anything, args.req.GameID).Return(nil)
		fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).Return(mockUsers, nil)
		fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).Return(game, nil)
		fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, round.RoundNum).Return(round, nil)
		fs.repo.On("GetMultiplayerRoundGuesses", mock.Anything, round.ID).Return(endRoundResponse, nil)
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []multiplayerEntity.Guess
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully end round",
			args: args{
				req: endRoundRequest,
			},
			setup: func(fs fields, args args) {
				mockGame := multiplayerEntity.Game{
					ID:           args.req.GameID,
					RoundCurrent: 1,
					TimerSeconds: 30,
					Players:      2,
					Provider:     "google",
				}

				mockRound := multiplayerEntity.Round{
					ID:           uuid.Must(uuid.NewV7()),
					RoundNum:     1,
					GuessesCount: 2,
					Finished:     false,
					StartedAt:    args.req.RequestTime.Add(-31 * time.Second),
				}

				setupCommonMocks(fs, args, mockRound, mockGame)

				fs.repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
					RequestTime: args.req.RequestTime,
					RoundID:     mockRound.ID,
				}).Return(nil)
			},
			want:    endRoundResponse,
			wantErr: assert.NoError,
		},
		{
			name: "successfully end round and game",
			args: args{
				req: endRoundRequest,
			},
			setup: func(fs fields, args args) {
				mockGame := multiplayerEntity.Game{
					ID:           args.req.GameID,
					RoundCurrent: 5,
					Rounds:       5,
					TimerSeconds: 30,
					Players:      2,
					Provider:     "google",
				}

				mockRound := multiplayerEntity.Round{
					ID:           uuid.Must(uuid.NewV7()),
					RoundNum:     5,
					GuessesCount: 2,
					Finished:     false,
					StartedAt:    args.req.RequestTime.Add(-31 * time.Second),
				}

				setupCommonMocks(fs, args, mockRound, mockGame)

				fs.repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
					RequestTime: args.req.RequestTime,
					RoundID:     mockRound.ID,
				}).Return(nil)

				fs.repo.On("EndMultiplayerGame", mock.Anything, dto.EndMultiplayerGameRequestDB{
					RequestTime: args.req.RequestTime,
					GameID:      args.req.GameID,
				}).Return(nil)
			},
			want:    endRoundResponse,
			wantErr: assert.NoError,
		},
		{
			name: "trying to end round that is already finished",
			args: args{
				req: endRoundRequest,
			},
			setup: func(fs fields, args args) {
				mockGame := multiplayerEntity.Game{
					ID:           args.req.GameID,
					RoundCurrent: 1,
					TimerSeconds: 30,
					Players:      2,
					Provider:     "google",
				}

				mockRound := multiplayerEntity.Round{
					ID:           uuid.Must(uuid.NewV7()),
					RoundNum:     1,
					GuessesCount: 2,
					Finished:     true,
					StartedAt:    args.req.RequestTime.Add(-31 * time.Second),
				}

				setupCommonMocks(fs, args, mockRound, mockGame)
			},
			want:    endRoundResponse,
			wantErr: assert.NoError,
		},
		{
			name: "trying to end round before timer has finished",
			args: args{
				req: endRoundRequest,
			},
			setup: func(fs fields, args args) {
				mockGame := multiplayerEntity.Game{
					ID:           args.req.GameID,
					RoundCurrent: 1,
					TimerSeconds: 30,
					Players:      2,
					Provider:     "google",
				}

				mockRound := multiplayerEntity.Round{
					ID:           uuid.Must(uuid.NewV7()),
					RoundNum:     1,
					GuessesCount: 1,
					Finished:     false,
					StartedAt:    args.req.RequestTime.Add(-29 * time.Second),
				}

				setupCommonMocks(fs, args, mockRound, mockGame)
			},
			want: nil,
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, multiplayerEntity.ErrRoundIsStillActive)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			fs := fields{
				repo: repo,
				pano: pano,
			}
			tt.setup(fs, tt.args)

			uc := multiplayer.NewUsecase(multiplayer.Config{}, repo, pano)

			got, err := uc.EndRound(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

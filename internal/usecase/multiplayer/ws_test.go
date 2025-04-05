package multiplayer_test

import (
	"context"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	multiplayerEntity "github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUsecase_NewMultiplayerRoundGuess(t *testing.T) {
	t.Parallel()

	saveGuessReq := dto.NewMultiplayerRoundGuessRequest{
		RequestTime: time.Now().UTC(),
		UserID:      1,
		GameID:      123,
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
							PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: 2, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				roundResponse := multiplayerEntity.Round{
					ID:       1,
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
							PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: 2, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				roundResponse := multiplayerEntity.Round{
					ID:       1,
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

	endRoundReq := dto.EndMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      1,
		UserID:      1,
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

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.EndMultiplayerRoundRequest
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
				req: endRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockMultiplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).
					Return([]user.MultiplayerUser{
						{
							PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: 2, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						RoundCurrent: 1,
						TimerSeconds: 30,
						Players:      2,
						Provider:     "google",
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 1).
					Return(multiplayerEntity.Round{
						ID:           1,
						RoundNum:     1,
						GuessesCount: 2,
						Finished:     false,
						StartedAt:    args.req.RequestTime.Add(-31 * time.Second),
					}, nil)

				fs.repo.On("GetMultiplayerRoundGuesses", mock.Anything, 1).
					Return(endRoundResponse, nil)

				fs.repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
					RequestTime: args.req.RequestTime,
					RoundID:     1,
				}).Return(nil)
			},
			want:    endRoundResponse,
			wantErr: assert.NoError,
		},
		{
			name: "trying to end round that is already finished",
			args: args{
				req: endRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockMultiplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).
					Return([]user.MultiplayerUser{
						{
							PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: 2, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						RoundCurrent: 1,
						TimerSeconds: 30,
						Players:      2,
						Provider:     "google",
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 1).
					Return(multiplayerEntity.Round{
						ID:           1,
						RoundNum:     1,
						GuessesCount: 2,
						Finished:     true,
						StartedAt:    args.req.RequestTime.Add(-31 * time.Second),
					}, nil)

				fs.repo.On("GetMultiplayerRoundGuesses", mock.Anything, 1).
					Return(endRoundResponse, nil)
			},
			want:    endRoundResponse,
			wantErr: assert.NoError,
		},
		{
			name: "trying to end round before timer has finished",
			args: args{
				req: endRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockMultiplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).
					Return([]user.MultiplayerUser{
						{
							PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: 2, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						RoundCurrent: 1,
						TimerSeconds: 30,
						Players:      2,
						Provider:     "google",
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 1).
					Return(multiplayerEntity.Round{
						ID:           1,
						RoundNum:     1,
						GuessesCount: 1,
						Finished:     false,
						StartedAt:    args.req.RequestTime.Add(-29 * time.Second),
					}, nil)

				fs.repo.On("GetMultiplayerRoundGuesses", mock.Anything, 1).
					Return(endRoundResponse, nil)
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

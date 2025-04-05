package singleplayer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	singleplayerEntity "github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUsecase_NewGame(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	createGameReq := dto.NewSingleplayerGameRequest{
		RequestTime:     now,
		UserID:          1,
		Rounds:          5,
		TimerSeconds:    60,
		Provider:        "google",
		MovementAllowed: true,
	}

	createdGame := singleplayerEntity.Game{
		ID:              123,
		RoundCurrent:    0,
		UserID:          createGameReq.UserID,
		Rounds:          createGameReq.Rounds,
		TimerSeconds:    createGameReq.TimerSeconds,
		MovementAllowed: createGameReq.MovementAllowed,
		Provider:        game.PanoramaProvider(createGameReq.Provider),
		Score:           0,
		Finished:        false,
		CreatedAt:       now,
	}

	type fields struct {
		cfg         singleplayer.Config
		repo        *mocks.Repository
		panoUsecase *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.NewSingleplayerGameRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully create new game",
			args: args{
				req: createGameReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("NewSingleplayerGame", mock.Anything, args.req).
					Return(createdGame.ID, nil)

				fs.repo.On("LockSingleplayerGame", mock.Anything, createdGame.ID).
					Return(nil)

				fs.repo.On("GetSingleplayerGame", mock.Anything, createdGame.ID).
					Return(createdGame, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, createdGame.ID, 0).
					Return(singleplayerEntity.Round{}, singleplayerEntity.ErrRoundNotFound)

				createdPanoID := 12341
				createdStreetviewID := "some_streetview_id"

				fs.panoUsecase.On("NewStreetview", mock.Anything, game.PanoramaProvider(args.req.Provider)).
					Return(game.PanoramaMetadata{
						ID:           createdPanoID,
						StreetviewID: createdStreetviewID,
					}, nil)

				fs.repo.On("NewSingleplayerRound", mock.Anything, dto.NewSingleplayerRoundDBRequest{
					CreatedAt:  args.req.RequestTime,
					StartedAt:  args.req.RequestTime.Add(fs.cfg.RoundStartDelay),
					LocationID: createdPanoID,
					GameID:     createdGame.ID,
					RoundNum:   1,
				}).Return(
					singleplayerEntity.Round{ID: 1, GameID: createdGame.ID},
					nil,
				)
			},
			want:    createdGame.ID,
			wantErr: assert.NoError,
		},
		{
			name: "error creating game - tx error",
			args: args{
				req: dto.NewSingleplayerGameRequest{
					UserID:      1,
					RequestTime: now,
				},
			},
			setup: func(fs fields, _ args) {
				fs.repo.On("RunTx", mock.Anything, mock.Anything).
					Return(errors.New("tx error"))
			},
			want:    0,
			wantErr: assert.Error,
		},
		{
			name: "error creating round - game lock error",
			args: args{
				req: dto.NewSingleplayerGameRequest{
					UserID:      1,
					RequestTime: now,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("NewSingleplayerGame", mock.Anything, args.req).
					Return(123, nil)

				fs.repo.On("LockSingleplayerGame", mock.Anything, createdGame.ID).
					Return(errors.New("game lock error"))
			},
			want:    0,
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoramaUsecase := mocks.NewPanoramaUsecase(t)
			cfg := singleplayer.Config{
				RoundStartDelay: 5 * time.Second,
			}
			fs := fields{repo: repo, panoUsecase: panoramaUsecase, cfg: cfg}
			uc := singleplayer.NewUsecase(cfg, repo, panoramaUsecase)

			tt.setup(fs, tt.args)

			got, err := uc.NewGame(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetGame(t *testing.T) {
	t.Parallel()

	validGame := singleplayerEntity.Game{
		ID:           123,
		UserID:       1,
		RoundCurrent: 1,
		Rounds:       5,
	}

	type fields struct {
		repo        *mocks.Repository
		panoUsecase *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.GetSingleplayerGameRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    singleplayerEntity.Game
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get game",
			args: args{
				req: dto.GetSingleplayerGameRequest{
					GameID: 123,
					UserID: 1,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(validGame, nil)
			},
			want:    validGame,
			wantErr: assert.NoError,
		},
		{
			name: "error getting game",
			args: args{
				req: dto.GetSingleplayerGameRequest{
					GameID: 123,
					UserID: 1,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{}, errors.New("db error"))
			},
			want:    singleplayerEntity.Game{},
			wantErr: assert.Error,
		},
		{
			name: "wrong user ID",
			args: args{
				req: dto.GetSingleplayerGameRequest{
					GameID: 123,
					UserID: 2,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(validGame, nil)
			},
			want: singleplayerEntity.Game{},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameWrongUserID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoramaUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, panoUsecase: panoramaUsecase}
			tt.setup(fs, tt.args)

			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoramaUsecase)

			got, err := uc.GetGame(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_EndGame(t *testing.T) {
	t.Parallel()

	validGame := singleplayerEntity.Game{
		ID:           123,
		UserID:       1,
		RoundCurrent: 5,
		Rounds:       5,
	}

	finishedRound := singleplayerEntity.Round{
		Finished: true,
	}

	type fields struct {
		repo        *mocks.Repository
		panoUsecase *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.EndSingleplayerGameRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully end game",
			args: args{
				req: dto.EndSingleplayerGameRequest{
					RequestTime: time.Now().UTC(),
					GameID:      123,
					UserID:      1,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(validGame, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, validGame.RoundCurrent).
					Return(finishedRound, nil)

				fs.repo.On("EndSingleplayerGame", mock.Anything, dto.EndSingleplayerGameRequestDB{
					RequestTime: args.req.RequestTime,
					GameID:      args.req.GameID,
				}).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "game not found",
			args: args{
				req: dto.EndSingleplayerGameRequest{
					GameID: 123,
					UserID: 1,
				},
			},
			setup: func(fs fields, _ args) {
				fs.repo.On("RunTx", mock.Anything, mock.Anything).
					Return(errors.New("tx error"))
			},
			wantErr: assert.Error,
		},
		{
			name: "wrong user ID",
			args: args{
				req: dto.EndSingleplayerGameRequest{
					GameID: 123,
					UserID: 2,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(validGame, nil)
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameWrongUserID)
			},
		},
		{
			name: "game still active",
			args: args{
				req: dto.EndSingleplayerGameRequest{
					GameID: 123,
					UserID: 1,
				},
			},
			setup: func(fs fields, args args) {
				invalidGame := validGame
				invalidGame.RoundCurrent = 3

				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(invalidGame, nil)
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameIsStillActive)
			},
		},
		{
			name: "round not finished",
			args: args{
				req: dto.EndSingleplayerGameRequest{
					GameID: 123,
					UserID: 1,
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})
				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(validGame, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, validGame.RoundCurrent).
					Return(singleplayerEntity.Round{Finished: false}, nil)
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrRoundIsStillActive)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, panoUsecase: panoUsecase}
			tt.setup(fs, tt.args)

			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoUsecase)

			err := uc.EndGame(t.Context(), tt.args.req)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_GetGames(t *testing.T) {
	t.Parallel()

	mockGames := []singleplayerEntity.Game{
		{ID: 1, UserID: 1},
		{ID: 2, UserID: 1},
	}

	type fields struct {
		repo        *mocks.Repository
		panoUsecase *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.GetSingleplayerGamesRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []singleplayerEntity.Game
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get games",
			args: args{
				req: dto.GetSingleplayerGamesRequest{UserID: 1},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetSingleplayerGames", mock.Anything, args.req).
					Return(mockGames, len(mockGames), nil)
			},
			want:    mockGames,
			wantErr: assert.NoError,
		},
		{
			name: "error getting games",
			args: args{
				req: dto.GetSingleplayerGamesRequest{UserID: 1},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetSingleplayerGames", mock.Anything, args.req).
					Return(nil, 0, errors.New("db error"))
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, panoUsecase: panoUsecase}
			tt.setup(fs, tt.args)

			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoUsecase)

			gotGames, gotAmountOfGames, err := uc.GetGames(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, gotGames)
			assert.Equal(t, len(tt.want), gotAmountOfGames)
		})
	}
}

package singleplayer_test

import (
	"context"
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

func TestUsecase_NewRound(t *testing.T) {
	t.Parallel()

	newRoundReq := dto.NewSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      123,
		UserID:      456,
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.NewSingleplayerRoundRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    singleplayerEntity.Round
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully create round",
			args: args{
				req: newRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockSingleplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       5,
						RoundCurrent: 1,
						Finished:     false,
					}, nil)

				fs.pano.On("NewStreetview", mock.Anything, mock.Anything).
					Return(game.PanoramaMetadata{}, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, 1).
					Return(singleplayerEntity.Round{
						ID:       1,
						GameID:   args.req.GameID,
						Finished: true,
						RoundNum: 1,
					}, nil)

				fs.repo.On("NewSingleplayerRound", mock.Anything, mock.Anything).
					Return(singleplayerEntity.Round{
						ID:       2,
						GameID:   args.req.GameID,
						Finished: false,
						RoundNum: 2,
					}, nil)
			},
			want: singleplayerEntity.Round{
				ID:       2,
				GameID:   newRoundReq.GameID,
				Finished: false,
				RoundNum: 2,
			},
			wantErr: assert.NoError,
		},
		{
			name: "user trying to generate new round for game of another user",
			args: args{
				req: newRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockSingleplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       777,
						Rounds:       5,
						RoundCurrent: 3,
						Finished:     false,
					}, nil)
			},
			want: singleplayerEntity.Round{},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameWrongUserID)
			},
		},
		{
			name: "trying to generate a new round above the maximum number of rounds",
			args: args{
				req: newRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockSingleplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       5,
						RoundCurrent: 5,
						Finished:     false,
					}, nil)
			},
			want:    singleplayerEntity.Round{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, pano: panoUsecase}
			tt.setup(fs, tt.args)

			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoUsecase)

			got, err := uc.NewRound(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetRound(t *testing.T) {
	t.Parallel()

	getRoundReq := dto.GetSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      1,
		UserID:      1,
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.GetSingleplayerRoundRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    singleplayerEntity.Round
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get round",
			args: args{
				req: getRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockSingleplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       5,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, 2).
					Return(singleplayerEntity.Round{
						ID:       1,
						GameID:   args.req.GameID,
						Finished: false,
						RoundNum: 2,
					}, nil)
			},
			want: singleplayerEntity.Round{
				ID:       1,
				GameID:   getRoundReq.GameID,
				Finished: false,
				RoundNum: 2,
			},
			wantErr: assert.NoError,
		},
		{
			name: "user trying to get another user's round",
			args: args{
				req: getRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockSingleplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       234,
						Rounds:       5,
						RoundCurrent: 3,
						Finished:     false,
					}, nil)
			},
			want: singleplayerEntity.Round{},
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, singleplayerEntity.ErrGameWrongUserID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, pano: panoUsecase}
			tt.setup(fs, tt.args)

			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoUsecase)

			got, err := uc.GetRound(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_EndRound(t *testing.T) {
	t.Parallel()

	endRoundReq := dto.EndSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      1,
		UserID:      1,
		Guess: game.LatLng{
			Lat: 1,
			Lng: 1,
		},
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.EndSingleplayerRoundRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    dto.EndCurrentRoundResponse
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

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       5,
						TimerSeconds: 60,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, 2).
					Return(singleplayerEntity.Round{
						ID:        1,
						GameID:    args.req.GameID,
						Lat:       11.22,
						Lng:       33.44,
						Finished:  false,
						RoundNum:  2,
						StartedAt: args.req.RequestTime.Add(-59 * time.Second),
					}, nil)

				fs.pano.On("CalculateScoreAndDistance", mock.Anything,
					11.22, 33.44, args.req.Guess.Lat, args.req.Guess.Lng).
					Return(1234, 5678, nil)

				fs.repo.On("NewSingleplayerRoundGuess", mock.Anything, dto.NewSingleplayerRoundGuessRequest{
					RequestTime: args.req.RequestTime,
					RoundID:     1,
					GameID:      args.req.GameID,
					Guess:       args.req.Guess,
					Score:       1234,
					Distance:    5678,
				}).
					Return(nil)
			},
			want: dto.EndCurrentRoundResponse{
				Score:    1234,
				Distance: 5678,
			},
			wantErr: assert.NoError,
		},
		{
			name: "trying to end already finished round",
			args: args{
				req: endRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       5,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, 2).
					Return(singleplayerEntity.Round{
						ID:       1,
						GameID:   args.req.GameID,
						RoundNum: 2,
						Finished: true,
					}, nil)
			},
			want: dto.EndCurrentRoundResponse{},
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, singleplayerEntity.ErrRoundAlreadyFinished)
			},
		},
		{
			name: "trying to end another user's round",
			args: args{
				req: endRoundReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       2,
						Rounds:       5,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)
			},
			want: dto.EndCurrentRoundResponse{},
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, singleplayerEntity.ErrGameWrongUserID)
			},
		},
		{
			name: "trying to end round after timer has expired",
			args: args{},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						TimerSeconds: 60,
						Rounds:       5,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, 2).
					Return(singleplayerEntity.Round{
						ID:        2,
						GameID:    args.req.GameID,
						Lat:       11.22,
						Lng:       33.44,
						Finished:  false,
						RoundNum:  2,
						StartedAt: args.req.RequestTime.Add(-61 * time.Second),
					}, nil)

				fs.pano.On("CalculateScoreAndDistance", mock.Anything,
					11.22, 33.44, args.req.Guess.Lat, args.req.Guess.Lng).
					Return(1234, 5678)

				fs.repo.On("NewSingleplayerRoundGuess", mock.Anything, dto.NewSingleplayerRoundGuessRequest{
					RequestTime: args.req.RequestTime,
					RoundID:     2,
					GameID:      args.req.GameID,
					Guess:       args.req.Guess,
					Score:       0,
					Distance:    5678,
				}).Return(nil)
			},
			want:    dto.EndCurrentRoundResponse{Score: 0, Distance: 5678},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, pano: panoUsecase}
			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoUsecase)

			tt.setup(fs, tt.args)

			got, err := uc.EndRound(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetGameRounds(t *testing.T) {
	t.Parallel()

	getRoundsReq := dto.GetSingleplayerGameRoundsRequest{
		RequestTime: time.Now().UTC(),
		GameID:      1,
		UserID:      1,
	}

	getRoundsResp := []singleplayerEntity.Guess{
		{
			RoundNum:     1,
			Score:        1234,
			MissDistance: 5678,
		},
		{
			RoundNum:     2,
			Score:        2345,
			MissDistance: 6789,
		},
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.GetSingleplayerGameRoundsRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []singleplayerEntity.Guess
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get game rounds",
			args: args{
				req: getRoundsReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       2,
						RoundCurrent: 2,
						Finished:     true,
					}, nil)

				fs.repo.On("GetSingleplayerGameGuesses", mock.Anything, args.req.GameID).
					Return(getRoundsResp, nil)
			},
			want:    getRoundsResp,
			wantErr: assert.NoError,
		},
		{
			name: "trying to get rounds of another user",
			args: args{
				req: getRoundsReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       2,
						Rounds:       2,
						RoundCurrent: 2,
						Finished:     true,
					}, nil)
			},
			want: nil,
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameWrongUserID)
			},
		},
		{
			name: "trying to get guesses of a game in progress",
			args: args{
				req: getRoundsReq,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       2,
						RoundCurrent: 1,
						Finished:     false,
					}, nil)
			},
			want: nil,
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameIsStillActive)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			panoUsecase := mocks.NewPanoramaUsecase(t)
			fs := fields{repo: repo, pano: panoUsecase}
			tt.setup(fs, tt.args)

			uc := singleplayer.NewUsecase(singleplayer.Config{}, repo, panoUsecase)

			got, err := uc.GetGameRounds(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

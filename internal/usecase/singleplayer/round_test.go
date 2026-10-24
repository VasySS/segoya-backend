package singleplayer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	singleplayerEntity "github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer/mocks"
)

func TestUsecase_NewRound(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	gameID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	panoramaID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	roundID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	lookupError := errors.New("round lookup failed")
	cfg := singleplayer.Config{RoundStartDelay: 3500 * time.Millisecond}

	tests := []struct {
		name          string
		current       int
		rounds        int
		wrongUser     bool
		roundFinished bool
		lookupError   error
		wantReuse     bool
		wantErr       error
	}{
		{name: "reuse unfinished round", current: 1, rounds: 5, wantReuse: true},
		{name: "create after finished round", current: 1, rounds: 5, roundFinished: true},
		{name: "create after missing round", rounds: 5, lookupError: singleplayerEntity.ErrRoundNotFound},
		{
			name: "limit rejects before current round lookup", current: 5, rounds: 5,
			wantErr: singleplayerEntity.ErrRoundMaxAmount,
		},
		{
			name: "ownership rejects before limit check", current: 5, rounds: 5, wrongUser: true,
			wantErr: singleplayerEntity.ErrGameWrongUserID,
		},
		{name: "zero rounds rejected", wantErr: game.ErrInvalidRounds},
		{name: "excess rounds rejected", current: 6, rounds: 5, wantErr: game.ErrInvalidState},
		{
			name: "lookup failure prevents creation", current: 1, rounds: 5,
			lookupError: lookupError, wantErr: lookupError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			uc := singleplayer.NewUsecase(cfg, repo, pano)
			req := dto.NewSingleplayerRoundRequest{RequestTime: now, GameID: gameID, UserID: userID}

			storedGame := singleplayerEntity.Game{
				ID: gameID, UserID: userID, Provider: game.GoogleProvider,
				Rounds: tt.rounds, RoundCurrent: tt.current,
			}
			if tt.wrongUser {
				storedGame.UserID = uuid.Nil
			}

			repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
				Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) }).Once()

			lock := repo.On("LockSingleplayerGame", mock.Anything, gameID).Return(nil).Once()
			fetchGame := repo.On("GetSingleplayerGame", mock.Anything, gameID).Return(storedGame, nil).Once()
			mock.InOrder(lock, fetchGame)

			var want singleplayerEntity.Round

			if !tt.wrongUser &&
				!errors.Is(tt.wantErr, singleplayerEntity.ErrRoundMaxAmount) &&
				!errors.Is(tt.wantErr, game.ErrInvalidRounds) &&
				!errors.Is(tt.wantErr, game.ErrInvalidState) {
				currentRound := singleplayerEntity.Round{
					ID: roundID, GameID: gameID, RoundNum: tt.current, Finished: tt.roundFinished,
					StartedAt: now.Add(-time.Hour),
					Lat:       1, Lng: 2,
				}
				fetchRound := repo.On("GetSingleplayerRound", mock.Anything, gameID, tt.current).
					Return(currentRound, tt.lookupError).Once()
				mock.InOrder(fetchGame, fetchRound)

				switch {
				case tt.wantReuse:
					want = currentRound
				case tt.wantErr == nil:
					want = singleplayerEntity.Round{
						ID: roundID, GameID: gameID, RoundNum: tt.current + 1,
						CreatedAt: now, StartedAt: now.Add(cfg.RoundStartDelay),
					}
					generate := pano.On("NewStreetview", mock.Anything, game.GoogleProvider).
						Return(game.PanoramaMetadata{ID: panoramaID}, nil).Once()
					create := repo.On("NewSingleplayerRound", mock.Anything, dto.NewSingleplayerRoundDBRequest{
						GameID: gameID, LocationID: panoramaID, RoundNum: tt.current + 1,
						CreatedAt: now, StartedAt: now.Add(cfg.RoundStartDelay),
					}).Return(want, nil).Once()
					mock.InOrder(fetchRound, generate, create)
				}
			}

			got, err := uc.NewRound(t.Context(), req)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, want, got)
		})
	}
}

func TestUsecase_GetRound(t *testing.T) {
	t.Parallel()

	getRoundReq := dto.GetSingleplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      uuid.Must(uuid.NewV7()),
		UserID:      uuid.Must(uuid.NewV7()),
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.GetSingleplayerRoundRequest
	}

	roundID := uuid.Must(uuid.NewV7())

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
						Provider:     "google",
						ID:           args.req.GameID,
						UserID:       args.req.UserID,
						Rounds:       5,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)

				fs.repo.On("GetSingleplayerRound", mock.Anything, args.req.GameID, 2).
					Return(singleplayerEntity.Round{
						ID:       roundID,
						GameID:   args.req.GameID,
						Finished: false,
						RoundNum: 2,
					}, nil)
			},
			want: singleplayerEntity.Round{
				ID:       roundID,
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
						Provider:     "google",
						ID:           args.req.GameID,
						UserID:       uuid.Must(uuid.NewV7()),
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

func TestUsecase_GetGameGuesses(t *testing.T) {
	t.Parallel()

	getRoundsReq := dto.GetSingleplayerGameGuessesRequest{
		RequestTime: time.Now().UTC(),
		GameID:      uuid.Must(uuid.NewV7()),
		UserID:      uuid.Must(uuid.NewV7()),
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
		req dto.GetSingleplayerGameGuessesRequest
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
						Provider:     "google",
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
						Provider:     "google",
						ID:           args.req.GameID,
						UserID:       uuid.Must(uuid.NewV7()),
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
						Provider:     "google",
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
		{
			name: "ownership rejects before active game check",
			args: args{req: getRoundsReq},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) })
				fs.repo.On("GetSingleplayerGame", mock.Anything, args.req.GameID).
					Return(singleplayerEntity.Game{Provider: "google", Rounds: 5, ID: args.req.GameID, UserID: uuid.Nil}, nil)
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, singleplayerEntity.ErrGameWrongUserID)
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

			got, err := uc.GetGameGuesses(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

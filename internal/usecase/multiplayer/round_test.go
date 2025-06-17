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

func TestUsecase_NewRound(t *testing.T) {
	t.Parallel()

	newRoundReq := dto.NewMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      uuid.Must(uuid.NewV7()),
		UserID:      uuid.Must(uuid.NewV7()),
	}

	createdPanoID := uuid.Must(uuid.NewV7())
	createdStreetviewID := "some_streetview_id"

	newRoundResp := multiplayerEntity.Round{
		ID:           uuid.Must(uuid.NewV7()),
		GameID:       newRoundReq.GameID,
		StreetviewID: createdStreetviewID,
		RoundNum:     2,
		CreatedAt:    newRoundReq.RequestTime,
		StartedAt:    newRoundReq.RequestTime.Add(5 * time.Second),
	}

	type fields struct {
		cfg  multiplayer.Config
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.NewMultiplayerRoundRequest
	}

	roundID := uuid.Must(uuid.NewV7())

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    multiplayerEntity.Round
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully create new round",
			args: args{
				req: newRoundReq,
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
							PublicProfile: user.PublicProfile{ID: newRoundReq.UserID, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						CreatorID:    args.req.UserID,
						Provider:     "google",
						Players:      2,
						Rounds:       5,
						RoundCurrent: 1,
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 1).
					Return(multiplayerEntity.Round{
						ID:       roundID,
						RoundNum: 2,
						Finished: true,
					}, nil)

				fs.pano.On("NewStreetview", mock.Anything, game.PanoramaProvider("google")).
					Return(game.PanoramaMetadata{
						ID:           createdPanoID,
						StreetviewID: createdStreetviewID,
						LatLng:       game.LatLng{Lat: 12.34, Lng: 45.67},
					}, nil)

				fs.repo.On("NewMultiplayerRound", mock.Anything, dto.NewMultiplayerRoundRequestDB{
					GameID:     args.req.GameID,
					LocationID: createdPanoID,
					RoundNum:   2,
					CreatedAt:  args.req.RequestTime,
					StartedAt:  args.req.RequestTime.Add(fs.cfg.RoundStartDelay),
				}).Return(newRoundResp, nil)
			},
			want:    newRoundResp,
			wantErr: assert.NoError,
		},
		{
			name: "user trying to generate new round for a game he is not a participant of",
			args: args{
				req: newRoundReq,
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
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"},
							Connected:     true,
						},
					}, nil)
			},
			want: multiplayerEntity.Round{},
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, multiplayerEntity.ErrGameWrongUserID)
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

				fs.repo.On("LockMultiplayerGame", mock.Anything, args.req.GameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.req.GameID).
					Return([]user.MultiplayerUser{
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: newRoundReq.UserID, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						CreatorID:    args.req.UserID,
						Provider:     "google",
						Players:      2,
						Rounds:       5,
						RoundCurrent: 5,
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 5).
					Return(multiplayerEntity.Round{
						ID:       uuid.Must(uuid.NewV7()),
						RoundNum: 5,
						Finished: true,
					}, nil)
			},
			want: multiplayerEntity.Round{},
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, multiplayerEntity.ErrRoundMaxAmount)
			},
		},
		{
			name: "trying to generate a new round when delay after last round has not passed",
			args: args{
				req: newRoundReq,
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
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: newRoundReq.UserID, Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						CreatorID:    args.req.UserID,
						Provider:     "google",
						Players:      2,
						Rounds:       5,
						RoundCurrent: 3,
						TimerSeconds: 60,
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 3).
					Return(multiplayerEntity.Round{
						ID:       roundID,
						RoundNum: 3,
						Finished: true,
						EndedAt:  args.req.RequestTime.Add(-9 * time.Second),
					}, nil)
			},
			want: multiplayerEntity.Round{
				ID:       roundID,
				RoundNum: 3,
				Finished: true,
				EndedAt:  newRoundReq.RequestTime.Add(-9 * time.Second),
			},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			cfg := multiplayer.Config{
				RoundStartDelay: 5 * time.Second,
				RoundEndDelay:   10 * time.Second,
			}
			fs := fields{
				cfg:  cfg,
				repo: repo,
				pano: pano,
			}
			tt.setup(fs, tt.args)

			uc := multiplayer.NewUsecase(cfg, repo, pano)

			got, err := uc.NewRound(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetRound(t *testing.T) {
	t.Parallel()

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.GetMultiplayerRoundRequest
	}

	roundID := uuid.Must(uuid.NewV7())

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    multiplayerEntity.Round
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get round",
			args: args{
				req: dto.GetMultiplayerRoundRequest{
					GameID: uuid.Must(uuid.NewV7()),
					UserID: uuid.Must(uuid.NewV7()),
				},
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
							PublicProfile: user.PublicProfile{ID: args.req.UserID, Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"},
							Connected:     true,
						},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           args.req.GameID,
						CreatorID:    args.req.UserID,
						Provider:     "google",
						Players:      2,
						Rounds:       5,
						RoundCurrent: 3,
						TimerSeconds: 60,
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, args.req.GameID, 3).
					Return(multiplayerEntity.Round{
						ID:       roundID,
						RoundNum: 3,
					}, nil)
			},
			want: multiplayerEntity.Round{
				ID:       roundID,
				RoundNum: 3,
			},
			wantErr: assert.NoError,
		},
		{
			name: "trying to get round of a game where user is not participanting",
			args: args{
				req: dto.GetMultiplayerRoundRequest{
					GameID: uuid.Must(uuid.NewV7()),
					UserID: uuid.Must(uuid.NewV7()),
				},
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
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username1"},
							Connected:     true,
						},
						{
							PublicProfile: user.PublicProfile{ID: uuid.Must(uuid.NewV7()), Username: "username2"},
							Connected:     true,
						},
					}, nil)
			},
			want: multiplayerEntity.Round{},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, multiplayerEntity.ErrGameWrongUserID)
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

			got, err := uc.GetRound(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

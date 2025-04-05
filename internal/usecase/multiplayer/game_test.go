package multiplayer_test

import (
	"context"
	"errors"
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

func TestUsecase_NewGame(t *testing.T) {
	t.Parallel()

	createGameReq := dto.NewMultiplayerGameRequest{
		RequestTime: time.Now().UTC(),
		CreatorID:   1,
		ConnectedPlayers: []user.PublicProfile{
			{ID: 1, Username: "username1"},
			{ID: 2, Username: "username2"},
		},
		Rounds:          4,
		TimerSeconds:    30,
		MovementAllowed: true,
		Provider:        "google",
	}

	type fields struct {
		cfg  multiplayer.Config
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.NewMultiplayerGameRequest
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

				createdGameID := 123

				fs.repo.On("NewMultiplayerGame", mock.Anything, args.req).
					Return(createdGameID, nil)

				fs.repo.On("LockMultiplayerGame", mock.Anything, createdGameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, createdGameID).
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

				fs.repo.On("GetMultiplayerGame", mock.Anything, createdGameID).
					Return(multiplayerEntity.Game{
						ID:              createdGameID,
						CreatorID:       args.req.CreatorID,
						Players:         2,
						Rounds:          args.req.Rounds,
						RoundCurrent:    0,
						MovementAllowed: args.req.MovementAllowed,
						Provider:        game.PanoramaProvider(args.req.Provider),
						TimerSeconds:    args.req.TimerSeconds,
					}, nil)

				fs.repo.On("GetMultiplayerRound", mock.Anything, createdGameID, 0).
					Return(multiplayerEntity.Round{}, multiplayerEntity.ErrRoundNotFound)

				createdPanoID := 12341
				createdStreetviewID := "some_streetview_id"

				fs.pano.On("NewStreetview", mock.Anything, game.PanoramaProvider(args.req.Provider)).
					Return(game.PanoramaMetadata{
						ID:           createdPanoID,
						StreetviewID: createdStreetviewID,
						LatLng:       game.LatLng{Lat: 12.34, Lng: 45.67},
					}, nil)

				fs.repo.On("NewMultiplayerRound", mock.Anything, dto.NewMultiplayerRoundRequestDB{
					GameID:     createdGameID,
					LocationID: createdPanoID,
					RoundNum:   1,
					CreatedAt:  args.req.RequestTime,
					StartedAt:  args.req.RequestTime.Add(fs.cfg.RoundStartDelay),
				}).Return(multiplayerEntity.Round{
					ID:           1,
					GameID:       createdGameID,
					StreetviewID: createdStreetviewID,
					RoundNum:     1,
					CreatedAt:    args.req.RequestTime,
					StartedAt:    args.req.RequestTime.Add(fs.cfg.RoundStartDelay),
				}, nil)
			},
			want:    123,
			wantErr: assert.NoError,
		},
		{
			name: "error creating game - tx error",
			args: args{
				req: createGameReq,
			},
			setup: func(fs fields, _ args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(_ context.Context, _ repository.TxFunc) error {
						return errors.New("tx error")
					})
			},
			want:    0,
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			cfg := multiplayer.Config{
				RoundStartDelay: 5 * time.Second,
			}
			fs := fields{
				cfg:  cfg,
				repo: repo,
				pano: pano,
			}
			tt.setup(fs, tt.args)

			uc := multiplayer.NewUsecase(cfg, repo, pano)

			got, err := uc.NewGame(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetGame(t *testing.T) {
	t.Parallel()

	validGame := multiplayerEntity.Game{
		ID:              123,
		CreatorID:       1,
		RoundCurrent:    1,
		Rounds:          5,
		MovementAllowed: true,
		Players:         2,
		TimerSeconds:    60,
		Provider:        "yandex",
		Finished:        false,
		CreatedAt:       time.Now().UTC().Add(-5 * time.Minute),
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		gameID int
		userID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    multiplayerEntity.Game
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get game",
			args: args{gameID: validGame.ID, userID: 1},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockMultiplayerGame", mock.Anything, args.gameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.gameID).
					Return([]user.MultiplayerUser{
						{PublicProfile: user.PublicProfile{ID: 1, Username: "username"}},
						{PublicProfile: user.PublicProfile{ID: 2, Username: "username2"}},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.gameID).
					Return(validGame, nil)
			},
			want:    validGame,
			wantErr: assert.NoError,
		},
		{
			name: "trying to get game with wrong user id",
			args: args{gameID: validGame.ID, userID: 333},
			setup: func(fs fields, args args) {
				fs.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				fs.repo.On("LockMultiplayerGame", mock.Anything, args.gameID).
					Return(nil)

				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.gameID).
					Return([]user.MultiplayerUser{
						{PublicProfile: user.PublicProfile{ID: 1, Username: "username"}},
						{PublicProfile: user.PublicProfile{ID: 2, Username: "username2"}},
					}, nil)
			},
			want: multiplayerEntity.Game{},
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

			got, err := uc.GetGame(t.Context(), tt.args.gameID, tt.args.userID)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_EndGame(t *testing.T) {
	t.Parallel()

	endGameReq := dto.EndMultiplayerGameRequest{
		RequestTime: time.Now().UTC(),
		GameID:      1,
		UserID:      1,
	}

	gameGuesses := []multiplayerEntity.Guess{
		{
			Username: "username1",
			RoundNum: 1,
		},
		{
			Username: "username1",
			RoundNum: 2,
		},
		{
			Username: "username2",
			RoundNum: 2,
		},
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		req dto.EndMultiplayerGameRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []multiplayerEntity.Guess
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully end game",
			args: args{
				req: endGameReq,
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
						{PublicProfile: user.PublicProfile{ID: 1, Username: "username1"}},
						{PublicProfile: user.PublicProfile{ID: 2, Username: "username2"}},
					}, nil)

				createdGameID := 1

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           createdGameID,
						Rounds:       2,
						RoundCurrent: 2,
						Finished:     false,
					}, nil)

				fs.repo.On("GetMultiplayerGameGuesses", mock.Anything, args.req.GameID).
					Return(gameGuesses, nil)

				fs.repo.On("EndMultiplayerGame", mock.Anything, dto.EndMultiplayerGameRequestDB{
					RequestTime: args.req.RequestTime,
					GameID:      createdGameID,
				}).
					Return(nil)
			},
			want:    gameGuesses,
			wantErr: assert.NoError,
		},
		{
			name: "trying to end game that is already finished",
			args: args{
				req: endGameReq,
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
						{PublicProfile: user.PublicProfile{ID: 1, Username: "username1"}},
						{PublicProfile: user.PublicProfile{ID: 2, Username: "username2"}},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           1,
						RoundCurrent: 2,
						Rounds:       2,
						Finished:     true,
					}, nil)

				fs.repo.On("GetMultiplayerGameGuesses", mock.Anything, args.req.GameID).
					Return(gameGuesses, nil)
			},
			want:    gameGuesses,
			wantErr: assert.NoError,
		},
		{
			name: "trying to end game that is still active",
			args: args{
				req: endGameReq,
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
						{PublicProfile: user.PublicProfile{ID: 1, Username: "username1"}},
						{PublicProfile: user.PublicProfile{ID: 2, Username: "username2"}},
					}, nil)

				fs.repo.On("GetMultiplayerGame", mock.Anything, args.req.GameID).
					Return(multiplayerEntity.Game{
						ID:           1,
						RoundCurrent: 1,
						Rounds:       2,
						Finished:     false,
					}, nil)
			},
			want: []multiplayerEntity.Guess(nil),
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(t, err, multiplayerEntity.ErrGameIsStillActive)
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

			guesses, err := uc.EndGame(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, guesses)
		})
	}
}

func TestUsecase_GameUser(t *testing.T) {
	t.Parallel()

	userResponse := user.MultiplayerUser{
		PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
		Connected:     true,
		Score:         11231,
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		userID int
		gameID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    user.MultiplayerUser
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get game user",
			args: args{
				userID: 1,
				gameID: 1,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetMultiplayerGameUser", mock.Anything, args.userID, args.gameID).
					Return(userResponse, nil)
			},
			want:    userResponse,
			wantErr: assert.NoError,
		},
		{
			name: "failed to get game user",
			args: args{
				userID: 2,
				gameID: 1,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetMultiplayerGameUser", mock.Anything, args.userID, args.gameID).
					Return(user.MultiplayerUser{}, errors.New("some db error"))
			},
			want:    user.MultiplayerUser{},
			wantErr: assert.Error,
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

			got, err := uc.GetGameUser(t.Context(), tt.args.userID, tt.args.gameID)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GameUsers(t *testing.T) {
	t.Parallel()

	usersResponse := []user.MultiplayerUser{
		{
			PublicProfile: user.PublicProfile{ID: 1, Username: "username1"},
			Connected:     true,
			Score:         11231,
		},
		{
			PublicProfile: user.PublicProfile{ID: 2, Username: "username2"},
			Connected:     true,
			Score:         23422,
		},
	}

	type fields struct {
		repo *mocks.Repository
		pano *mocks.PanoramaUsecase
	}

	type args struct {
		gameID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []user.MultiplayerUser
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get game users",
			args: args{
				gameID: 1,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetMultiplayerGameUsers", mock.Anything, args.gameID).
					Return(usersResponse, nil)
			},
			want:    usersResponse,
			wantErr: assert.NoError,
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

			got, err := uc.GetGameUsers(t.Context(), tt.args.gameID)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

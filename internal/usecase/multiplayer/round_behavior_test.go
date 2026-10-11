package multiplayer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	multiplayerEntity "github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer/mocks"
)

func TestUsecase_NewRoundReuseBoundaries(t *testing.T) {
	t.Parallel()

	endedAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	boundary := endedAt.Add(10 * time.Second)
	tests := []struct {
		name         string
		requestTime  time.Time
		finished     bool
		currentRound int
		missing      bool
		wantReuse    bool
		wantErr      error
	}{
		{
			name: "unfinished round reuses after delay", requestTime: boundary.Add(time.Nanosecond),
			currentRound: 2, wantReuse: true,
		},
		{
			name: "finished round reuses before delay", requestTime: boundary.Add(-time.Nanosecond),
			currentRound: 2, finished: true, wantReuse: true,
		},
		{name: "finished round replaced at delay", requestTime: boundary, currentRound: 2, finished: true},
		{
			name: "finished round replaced after delay", requestTime: boundary.Add(time.Nanosecond),
			currentRound: 2, finished: true,
		},
		{
			name: "unfinished final round reuses before maximum check", requestTime: boundary.Add(time.Nanosecond),
			currentRound: 5, wantReuse: true,
		},
		{
			name: "finished final round reuses before delay", requestTime: boundary.Add(-time.Nanosecond),
			currentRound: 5, finished: true, wantReuse: true,
		},
		{
			name: "finished final round rejects at delay", requestTime: boundary, currentRound: 5,
			finished: true, wantErr: multiplayerEntity.ErrRoundMaxAmount,
		},
		{name: "missing round creates next round", requestTime: boundary, currentRound: 2, missing: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			req := dto.NewMultiplayerRoundRequest{
				RequestTime: tt.requestTime,
				GameID:      uuid.New(),
				UserID:      uuid.New(),
			}
			currentGame := multiplayerEntity.Game{
				ID: req.GameID, Provider: game.GoogleProvider, Rounds: 5, RoundCurrent: tt.currentRound,
			}
			currentRound := multiplayerEntity.Round{
				ID: uuid.New(), GameID: req.GameID, RoundNum: tt.currentRound, Finished: tt.finished, EndedAt: endedAt,
			}

			repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
				Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) }).Once()

			lock := repo.On("LockMultiplayerGame", mock.Anything, req.GameID).Return(nil).Once()
			members := repo.On("GetMultiplayerGameUsers", mock.Anything, req.GameID).
				Return([]user.MultiplayerUser{{PublicProfile: user.PublicProfile{ID: req.UserID}}}, nil).Once()
			getGame := repo.On("GetMultiplayerGame", mock.Anything, req.GameID).Return(currentGame, nil).Once()

			var roundErr error

			if tt.missing {
				roundErr = multiplayerEntity.ErrRoundNotFound
			}

			getRound := repo.On("GetMultiplayerRound", mock.Anything, req.GameID, tt.currentRound).
				Return(currentRound, roundErr).Once()
			mock.InOrder(lock, members, getGame, getRound)

			wantRound := currentRound

			if !tt.wantReuse && tt.wantErr == nil {
				panorama := game.PanoramaMetadata{ID: uuid.New()}
				pano.On("NewStreetview", mock.Anything, game.GoogleProvider).Return(panorama, nil).Once()

				wantRound = multiplayerEntity.Round{ID: uuid.New(), RoundNum: tt.currentRound + 1}
				repo.On("NewMultiplayerRound", mock.Anything, dto.NewMultiplayerRoundRequestDB{
					GameID:     req.GameID,
					LocationID: panorama.ID,
					RoundNum:   tt.currentRound + 1,
					CreatedAt:  tt.requestTime,
					StartedAt:  tt.requestTime.Add(5 * time.Second),
				}).Return(wantRound, nil).Once()
			}

			uc := multiplayer.NewUsecase(multiplayer.Config{
				RoundStartDelay: 5 * time.Second, RoundEndDelay: 10 * time.Second,
			}, repo, pano)

			got, err := uc.NewRound(t.Context(), req)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, multiplayerEntity.Round{}, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, wantRound, got)
		})
	}
}

func TestUsecase_EndRoundBoundaries(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	boundary := startedAt.Add(30 * time.Second)
	tests := []struct {
		name         string
		requestTime  time.Time
		timerSeconds int
		guessesCount int
		finished     bool
		finalRound   bool
		wantErr      error
	}{
		{name: "no timer ends before round starts", requestTime: startedAt.Add(-time.Nanosecond)},
		{
			name: "before timer rejects partial guesses", requestTime: boundary.Add(-time.Nanosecond),
			timerSeconds: 30, guessesCount: 1, wantErr: multiplayerEntity.ErrRoundIsStillActive,
		},
		{name: "at timer ends with partial guesses", requestTime: boundary, timerSeconds: 30, guessesCount: 1},
		{
			name: "after timer ends with partial guesses", requestTime: boundary.Add(time.Nanosecond),
			timerSeconds: 30, guessesCount: 1,
		},
		{
			name: "all guessed ends before timer", requestTime: boundary.Add(-time.Nanosecond),
			timerSeconds: 30, guessesCount: 2,
		},
		{
			name: "extra guesses reject before timer", requestTime: boundary.Add(-time.Nanosecond),
			timerSeconds: 30, guessesCount: 3, wantErr: multiplayerEntity.ErrRoundIsStillActive,
		},
		{name: "final round also ends game", requestTime: boundary, timerSeconds: 30, finalRound: true},
		{
			name: "finished final round returns guesses before checking timer", requestTime: startedAt,
			timerSeconds: 30, finished: true, finalRound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			req := dto.EndMultiplayerRoundRequest{RequestTime: tt.requestTime, GameID: uuid.New(), UserID: uuid.New()}

			currentGame := multiplayerEntity.Game{
				ID: req.GameID, Rounds: 5, RoundCurrent: 2, TimerSeconds: tt.timerSeconds, Players: 2,
			}
			if tt.finalRound {
				currentGame.RoundCurrent = currentGame.Rounds
			}

			currentRound := multiplayerEntity.Round{
				ID: uuid.New(), RoundNum: currentGame.RoundCurrent, GuessesCount: tt.guessesCount,
				StartedAt: startedAt, Finished: tt.finished,
			}
			guesses := []multiplayerEntity.Guess{{Username: "player", Score: 3032}}

			repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
				Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) }).Once()

			lock := repo.On("LockMultiplayerGame", mock.Anything, req.GameID).Return(nil).Once()
			members := repo.On("GetMultiplayerGameUsers", mock.Anything, req.GameID).
				Return([]user.MultiplayerUser{{PublicProfile: user.PublicProfile{ID: req.UserID}}}, nil).Once()
			getGame := repo.On("GetMultiplayerGame", mock.Anything, req.GameID).Return(currentGame, nil).Once()
			getRound := repo.On("GetMultiplayerRound", mock.Anything, req.GameID, currentGame.RoundCurrent).
				Return(currentRound, nil).Once()
			getGuesses := repo.On("GetMultiplayerRoundGuesses", mock.Anything, currentRound.ID).
				Return(guesses, nil).Once()
			mock.InOrder(lock, members, getGame, getRound, getGuesses)

			if !tt.finished && tt.wantErr == nil {
				endRound := repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
					RequestTime: req.RequestTime, RoundID: currentRound.ID,
				}).Return(nil).Once()
				mock.InOrder(getGuesses, endRound)

				if tt.finalRound {
					endGame := repo.On("EndMultiplayerGame", mock.Anything, dto.EndMultiplayerGameRequestDB{
						RequestTime: req.RequestTime, GameID: req.GameID,
					}).Return(nil).Once()
					mock.InOrder(endRound, endGame)
				}
			}

			uc := multiplayer.NewUsecase(multiplayer.Config{}, repo, pano)

			got, err := uc.EndRound(t.Context(), req)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, guesses, got)
		})
	}
}

func TestUsecase_NewRoundGuessFixedScore(t *testing.T) {
	t.Parallel()

	for _, finished := range []bool{false, true} {
		name := "active round saves expected score and distance"
		if finished {
			name = "finished round rejects without writing"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			req := dto.NewMultiplayerRoundGuessRequest{
				RequestTime: time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC),
				GameID:      uuid.New(),
				UserID:      uuid.New(),
				Guess:       game.LatLng{Lat: 6.74637},
			}
			currentGame := multiplayerEntity.Game{ID: req.GameID, RoundCurrent: 2, Provider: game.GoogleProvider}
			currentRound := multiplayerEntity.Round{ID: uuid.New(), RoundNum: 2, Finished: finished}

			repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
				Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) }).Once()

			getGame := repo.On("GetMultiplayerGame", mock.Anything, req.GameID).Return(currentGame, nil).Once()
			members := repo.On("GetMultiplayerGameUsers", mock.Anything, req.GameID).
				Return([]user.MultiplayerUser{{PublicProfile: user.PublicProfile{ID: req.UserID}}}, nil).Once()
			getRound := repo.On("GetMultiplayerRound", mock.Anything, req.GameID, currentGame.RoundCurrent).
				Return(currentRound, nil).Once()
			mock.InOrder(getGame, members, getRound)

			if !finished {
				repo.On("NewMultiplayerRoundGuess", mock.Anything, dto.NewMultiplayerRoundGuessRequestDB{
					RequestTime: req.RequestTime,
					RoundID:     currentRound.ID,
					UserID:      req.UserID,
					Score:       3032,
					Distance:    750000,
					Lat:         req.Guess.Lat,
					Lng:         req.Guess.Lng,
				}).Return(nil).Once()
			}

			uc := multiplayer.NewUsecase(multiplayer.Config{}, repo, pano)

			err := uc.NewRoundGuess(t.Context(), req)
			if finished {
				require.ErrorIs(t, err, multiplayerEntity.ErrRoundAlreadyFinished)
				return
			}

			require.NoError(t, err)
		})
	}
}

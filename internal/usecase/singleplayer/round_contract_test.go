package singleplayer_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	sp "github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer/mocks"
)

type singleplayerHarness struct {
	uc       *singleplayer.Usecase
	repo     *mocks.Repository
	pano     *mocks.PanoramaUsecase
	now      time.Time
	game     sp.Game
	round    sp.Round
	metadata game.PanoramaMetadata
}

func newSingleplayerHarness(t *testing.T) *singleplayerHarness {
	t.Helper()
	h := &singleplayerHarness{
		repo: mocks.NewRepository(t), pano: mocks.NewPanoramaUsecase(t),
		now: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
		game: sp.Game{
			ID:           uuid.MustParse("019a553a-fc00-7000-8000-000000000002"),
			UserID:       uuid.MustParse("019a553a-fc00-7000-8000-000000000001"),
			Provider:     game.GoogleProvider,
			Rounds:       5,
			RoundCurrent: 2,
			TimerSeconds: 60,
		},
		round:    sp.Round{ID: uuid.MustParse("019a553a-fc00-7000-8000-000000000003"), RoundNum: 2},
		metadata: game.PanoramaMetadata{ID: uuid.MustParse("019a553a-fc00-7000-8000-000000000004")},
	}
	h.round.GameID, h.round.StartedAt = h.game.ID, h.now
	h.uc = singleplayer.NewUsecase(singleplayer.Config{RoundStartDelay: 5 * time.Second}, h.repo, h.pano)

	return h
}

func (h *singleplayerHarness) expectTransaction() {
	h.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
		Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) }).Once()
}

func TestUsecase_NewRoundFailures(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"transaction", "lock", "game", "panorama", "insert", "commit"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			h := newSingleplayerHarness(t)
			cause := errors.New("repository unavailable")
			setupSingleplayerNewRoundFailure(h, failure, cause)

			got, err := h.uc.NewRound(t.Context(), dto.NewSingleplayerRoundRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.game.UserID,
			})
			require.ErrorIs(t, err, cause)
			assert.Equal(t, sp.Round{}, got)
		})
	}
}

func TestUsecase_GetRoundFailures(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"transaction", "lock", "game", "round"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			h := newSingleplayerHarness(t)
			cause := errors.New("repository unavailable")
			setup := func() {
				if failure == "transaction" {
					h.repo.On("RunTx", mock.Anything, mock.Anything).Return(cause).Once()
					return
				}

				h.expectTransaction()

				if failure == "lock" {
					h.repo.On("LockSingleplayerGame", mock.Anything, h.game.ID).Return(cause).Once()
					return
				}

				h.repo.On("LockSingleplayerGame", mock.Anything, h.game.ID).Return(nil).Once()

				if failure == "game" {
					h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(sp.Game{}, cause).Once()
					return
				}

				h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()
				h.repo.On("GetSingleplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(sp.Round{}, cause).Once()
			}
			setup()

			got, err := h.uc.GetRound(t.Context(), dto.GetSingleplayerRoundRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.game.UserID,
			})
			require.ErrorIs(t, err, cause)
			assert.Equal(t, sp.Round{}, got)
		})
	}
}

func TestUsecase_EndRound(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                          string
		guess                         *game.LatLng
		score, distance               int
		elapsed                       time.Duration
		untimed, finished, wrongOwner bool
		want                          error
	}{
		{name: "no guess has zero score"},
		{name: "zero coordinates score normally", guess: &game.LatLng{}, score: 5000},
		{
			name:     "before deadline",
			guess:    &game.LatLng{Lat: 6.74637},
			elapsed:  time.Minute - time.Nanosecond,
			score:    3032,
			distance: 750000,
		},
		{name: "at deadline", guess: &game.LatLng{Lat: 6.74637}, elapsed: time.Minute, score: 3032, distance: 750000},
		{
			name:     "late guess keeps distance",
			guess:    &game.LatLng{Lat: 6.74637},
			elapsed:  time.Minute + time.Nanosecond,
			distance: 750000,
		},
		{
			name:     "timer disabled",
			guess:    &game.LatLng{Lat: 6.74637},
			elapsed:  time.Hour,
			untimed:  true,
			score:    3032,
			distance: 750000,
		},
		{name: "wrong owner rejects", wrongOwner: true, want: sp.ErrGameWrongUserID},
		{name: "finished round rejects", finished: true, want: sp.ErrRoundAlreadyFinished},
		{name: "before start rejects", guess: &game.LatLng{}, elapsed: -time.Nanosecond, want: game.ErrRoundNotStarted},
		{name: "no guess before start rejects", elapsed: -time.Nanosecond, want: game.ErrRoundNotStarted},
		{name: "nonfinite coordinates reject", guess: &game.LatLng{Lng: math.Inf(1)}, want: game.ErrInvalidCoordinates},
		{name: "out of range coordinates reject", guess: &game.LatLng{Lat: -91}, want: game.ErrInvalidCoordinates},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newSingleplayerHarness(t)

			req := dto.EndSingleplayerRoundRequest{
				RequestTime: h.now.Add(tt.elapsed),
				GameID:      h.game.ID,
				UserID:      h.game.UserID,
				Guess:       tt.guess,
			}

			if tt.wrongOwner {
				h.game.UserID = uuid.Nil
			}

			if tt.untimed {
				h.game.TimerSeconds = 0
			}

			h.round.Finished = tt.finished
			h.expectTransaction()

			fetchGame := h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()

			if !tt.wrongOwner {
				fetchRound := h.repo.On("GetSingleplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).
					Return(h.round, nil).Once()
				mock.InOrder(fetchGame, fetchRound)

				if tt.want == nil {
					write := h.repo.On("NewSingleplayerRoundGuess", mock.Anything, dto.NewSingleplayerRoundGuessRequest{
						RequestTime: req.RequestTime, GameID: h.game.ID, RoundID: h.round.ID,
						Guess: tt.guess, Score: tt.score, Distance: tt.distance,
					}).Return(nil).Once()
					mock.InOrder(fetchRound, write)
				}
			}

			got, err := h.uc.EndRound(t.Context(), req)
			require.ErrorIs(t, err, tt.want)
			assert.Equal(t, dto.EndCurrentRoundResponse{Score: tt.score, Distance: tt.distance}, got)

			if tt.want != nil {
				h.repo.AssertNotCalled(t, "NewSingleplayerRoundGuess", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestUsecase_EndRoundFailures(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"transaction", "game", "invalid game", "round", "write", "commit"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			h := newSingleplayerHarness(t)
			cause := errors.New("repository unavailable")
			setupSingleplayerEndRoundFailure(h, failure, cause)

			want := cause
			if failure == "invalid game" {
				want = game.ErrInvalidRounds
			}

			got, err := h.uc.EndRound(t.Context(), dto.EndSingleplayerRoundRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.game.UserID,
			})
			require.ErrorIs(t, err, want)
			assert.Equal(t, dto.EndCurrentRoundResponse{}, got)
		})
	}
}

func setupSingleplayerNewRoundFailure(h *singleplayerHarness, failure string, cause error) {
	if failure == "transaction" {
		h.repo.On("RunTx", mock.Anything, mock.Anything).Return(cause).Once()
		return
	}

	if failure == "commit" {
		h.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
			Return(func(ctx context.Context, fn repository.TxFunc) error {
				if err := fn(ctx); err != nil {
					return err
				}

				return cause
			}).Once()
	} else {
		h.expectTransaction()
	}

	if failure == "lock" {
		h.repo.On("LockSingleplayerGame", mock.Anything, h.game.ID).Return(cause).Once()
		return
	}

	h.repo.On("LockSingleplayerGame", mock.Anything, h.game.ID).Return(nil).Once()

	if failure == "game" {
		h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(sp.Game{}, cause).Once()
		return
	}

	h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()
	h.repo.On("GetSingleplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).
		Return(sp.Round{}, sp.ErrRoundNotFound).Once()

	if failure == "panorama" {
		h.pano.On("NewStreetview", mock.Anything, game.GoogleProvider).Return(game.PanoramaMetadata{}, cause).Once()
		return
	}

	h.pano.On("NewStreetview", mock.Anything, game.GoogleProvider).Return(h.metadata, nil).Once()

	var insertErr error
	if failure == "insert" {
		insertErr = cause
	}

	h.repo.On("NewSingleplayerRound", mock.Anything, dto.NewSingleplayerRoundDBRequest{
		GameID: h.game.ID, LocationID: h.metadata.ID, RoundNum: 3, CreatedAt: h.now, StartedAt: h.now.Add(5 * time.Second),
	}).Return(h.round, insertErr).Once()
}

func setupSingleplayerEndRoundFailure(h *singleplayerHarness, failure string, cause error) {
	if failure == "transaction" {
		h.repo.On("RunTx", mock.Anything, mock.Anything).Return(cause).Once()
		return
	}

	if failure == "commit" {
		h.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
			Return(func(ctx context.Context, fn repository.TxFunc) error {
				if err := fn(ctx); err != nil {
					return err
				}

				return cause
			}).Once()
	} else {
		h.expectTransaction()
	}

	if failure == "game" {
		h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(sp.Game{}, cause).Once()
		return
	}

	if failure == "invalid game" {
		h.game.Rounds = 0
	}

	h.repo.On("GetSingleplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()

	if failure == "invalid game" {
		return
	}

	if failure == "round" {
		h.repo.On("GetSingleplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(sp.Round{}, cause).Once()
		return
	}

	h.repo.On("GetSingleplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(h.round, nil).Once()

	var writeErr error
	if failure == "write" {
		writeErr = cause
	}

	h.repo.On("NewSingleplayerRoundGuess", mock.Anything, dto.NewSingleplayerRoundGuessRequest{
		RequestTime: h.now, GameID: h.game.ID, RoundID: h.round.ID,
	}).Return(writeErr).Once()
}

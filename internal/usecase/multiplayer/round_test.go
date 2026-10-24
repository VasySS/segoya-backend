package multiplayer_test

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
	mp "github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer/mocks"
)

type multiplayerHarness struct {
	uc       *multiplayer.Usecase
	repo     *mocks.Repository
	pano     *mocks.PanoramaUsecase
	config   multiplayer.Config
	now      time.Time
	userID   uuid.UUID
	game     mp.Game
	round    mp.Round
	metadata game.PanoramaMetadata
}

func newMultiplayerHarness(t *testing.T) *multiplayerHarness {
	t.Helper()
	h := &multiplayerHarness{
		repo: mocks.NewRepository(t), pano: mocks.NewPanoramaUsecase(t),
		config: multiplayer.Config{RoundStartDelay: 5 * time.Second, RoundEndDelay: 10 * time.Second},
		now:    time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
		userID: uuid.MustParse("019a553a-fc00-7000-8000-000000000001"),
		game: mp.Game{
			ID:           uuid.MustParse("019a553a-fc00-7000-8000-000000000002"),
			Provider:     game.GoogleProvider,
			Players:      2,
			Rounds:       5,
			RoundCurrent: 2,
		},
		round: mp.Round{ID: uuid.MustParse("019a553a-fc00-7000-8000-000000000003"), RoundNum: 2},
		metadata: game.PanoramaMetadata{
			ID:           uuid.MustParse("019a553a-fc00-7000-8000-000000000004"),
			StreetviewID: "panorama",
			LatLng:       game.LatLng{Lat: 12, Lng: 34},
		},
	}
	h.game.CreatorID = h.userID
	h.round.GameID, h.round.StartedAt, h.round.EndedAt = h.game.ID, h.now, h.now.Add(-h.config.RoundEndDelay)
	h.uc = multiplayer.NewUsecase(h.config, h.repo, h.pano)

	return h
}

func (h *multiplayerHarness) expectTransaction() {
	h.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
		Return(func(ctx context.Context, fn repository.TxFunc) error { return fn(ctx) }).Once()
}

func (h *multiplayerHarness) expectFailureTransaction(failure string, cause error) bool {
	if failure == "transaction" {
		h.repo.On("RunTx", mock.Anything, mock.Anything).Return(cause).Once()
		return false
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

	return true
}

func (h *multiplayerHarness) participants() []user.MultiplayerUser {
	return []user.MultiplayerUser{{PublicProfile: user.PublicProfile{ID: h.userID}, Connected: true}}
}

func TestUsecase_NewRound(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                     string
		current                  int
		finished, missing, reuse bool
		offset                   time.Duration
		want                     error
	}{
		{name: "unfinished round reuses after delay", current: 2, offset: time.Nanosecond, reuse: true},
		{name: "finished round reuses before delay", current: 2, finished: true, offset: -time.Nanosecond, reuse: true},
		{name: "finished round replaced at delay", current: 2, finished: true},
		{name: "finished round replaced after delay", current: 2, finished: true, offset: time.Nanosecond},
		{name: "unfinished final round reuses", current: 5, offset: time.Nanosecond, reuse: true},
		{name: "finished final round reuses before delay", current: 5, finished: true, offset: -time.Nanosecond, reuse: true},
		{name: "finished final round rejects at delay", current: 5, finished: true, want: mp.ErrRoundMaxAmount},
		{name: "missing round creates next round", current: 2, missing: true},
		{name: "first round creates", missing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)
			h.game.RoundCurrent, h.round.RoundNum, h.round.Finished = tt.current, tt.current, tt.finished
			req := dto.NewMultiplayerRoundRequest{RequestTime: h.now.Add(tt.offset), GameID: h.game.ID, UserID: h.userID}
			h.expectTransaction()
			lock := h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()
			members := h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(h.participants(), nil).Once()
			fetchGame := h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()

			var lookupErr error
			if tt.missing {
				lookupErr = mp.ErrRoundNotFound
			}

			fetchRound := h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, tt.current).
				Return(h.round, lookupErr).Once()
			mock.InOrder(lock, members, fetchGame, fetchRound)

			var want mp.Round
			if tt.reuse {
				want = h.round
			} else if tt.want == nil {
				generate := h.pano.On("NewStreetview", mock.Anything, game.GoogleProvider).Return(h.metadata, nil).Once()
				want = mp.Round{
					ID:        h.round.ID,
					GameID:    h.game.ID,
					RoundNum:  tt.current + 1,
					CreatedAt: req.RequestTime,
					StartedAt: req.RequestTime.Add(h.config.RoundStartDelay),
				}
				insert := h.repo.On("NewMultiplayerRound", mock.Anything, dto.NewMultiplayerRoundRequestDB{
					GameID: h.game.ID, LocationID: h.metadata.ID, RoundNum: tt.current + 1,
					CreatedAt: req.RequestTime, StartedAt: req.RequestTime.Add(h.config.RoundStartDelay),
				}).Return(want, nil).Once()
				mock.InOrder(fetchRound, generate, insert)
			}

			got, err := h.uc.NewRound(t.Context(), req)
			require.ErrorIs(t, err, tt.want)
			assert.Equal(t, want, got)
		})
	}
}

func TestUsecase_NewRoundFailures(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{
		"transaction",
		"lock",
		"participants",
		"not a participant",
		"game",
		"invalid game",
		"round",
		"panorama",
		"insert",
		"commit",
	} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)
			cause := errors.New("repository unavailable")

			want := cause
			if failure == "not a participant" {
				want = mp.ErrGameWrongUserID
			}

			if failure == "invalid game" {
				want = game.ErrInvalidPlayers
			}

			setupMultiplayerNewRoundFailure(h, failure, cause)

			got, err := h.uc.NewRound(t.Context(), dto.NewMultiplayerRoundRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.userID,
			})
			require.ErrorIs(t, err, want)
			assert.Equal(t, mp.Round{}, got)
		})
	}
}

func TestUsecase_GetRound(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"", "transaction", "lock", "participants", "not a participant", "game", "round"} {
		name := failure + " failure"
		if failure == "" {
			name = "success"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)
			cause := errors.New("repository unavailable")
			setupMultiplayerGetRound(h, failure, cause)

			got, err := h.uc.GetRound(t.Context(), dto.GetMultiplayerRoundRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.userID,
			})

			if failure != "" {
				want := cause
				if failure == "not a participant" {
					want = mp.ErrGameWrongUserID
				}

				require.ErrorIs(t, err, want)
				assert.Equal(t, mp.Round{}, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, h.round, got)
		})
	}
}

func setupMultiplayerNewRoundFailure(h *multiplayerHarness, failure string, cause error) {
	if !setupLockedMultiplayerGame(h, failure, cause) {
		return
	}

	if failure == "round" {
		h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, 2).Return(mp.Round{}, cause).Once()
		return
	}

	h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, 2).Return(mp.Round{}, mp.ErrRoundNotFound).Once()

	if failure == "panorama" {
		h.pano.On("NewStreetview", mock.Anything, game.GoogleProvider).Return(game.PanoramaMetadata{}, cause).Once()
		return
	}

	h.pano.On("NewStreetview", mock.Anything, game.GoogleProvider).Return(h.metadata, nil).Once()

	var insertErr error
	if failure == "insert" {
		insertErr = cause
	}

	h.repo.On("NewMultiplayerRound", mock.Anything, dto.NewMultiplayerRoundRequestDB{
		GameID: h.game.ID, LocationID: h.metadata.ID, RoundNum: 3,
		CreatedAt: h.now, StartedAt: h.now.Add(h.config.RoundStartDelay),
	}).Return(h.round, insertErr).Once()
}

func setupMultiplayerGetRound(h *multiplayerHarness, failure string, cause error) {
	if failure == "transaction" {
		h.repo.On("RunTx", mock.Anything, mock.Anything).Return(cause).Once()
		return
	}

	h.expectTransaction()

	if failure == "lock" {
		h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(cause).Once()
		return
	}

	h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()

	if failure == "participants" {
		h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(nil, cause).Once()
		return
	}

	if failure == "not a participant" {
		h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return([]user.MultiplayerUser{}, nil).Once()
		return
	}

	h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(h.participants(), nil).Once()

	if failure == "game" {
		h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(mp.Game{}, cause).Once()
		return
	}

	h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()

	var roundErr error
	if failure == "round" {
		roundErr = cause
	}

	h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(h.round, roundErr).Once()
}

func setupLockedMultiplayerGame(h *multiplayerHarness, failure string, cause error) bool {
	if !h.expectFailureTransaction(failure, cause) {
		return false
	}

	if failure == "lock" {
		h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(cause).Once()
		return false
	}

	h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()

	if failure == "participants" {
		h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(nil, cause).Once()
		return false
	}

	if failure == "not a participant" {
		h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return([]user.MultiplayerUser{}, nil).Once()
		return false
	}

	h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(h.participants(), nil).Once()

	if failure == "game" {
		h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(mp.Game{}, cause).Once()
		return false
	}

	if failure == "invalid game" {
		h.game.Players = 1
	}

	h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()

	return failure != "invalid game"
}

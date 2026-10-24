package multiplayer_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	mp "github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestUsecase_NewRoundGuess(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                   string
		guess                  *game.LatLng
		score, distance        int
		offset                 time.Duration
		finished, disconnected bool
		want                   error
	}{
		{name: "distant guess", guess: &game.LatLng{Lat: 6.74637}, score: 3032, distance: 750000},
		{name: "no guess has zero score"},
		{name: "explicit zero coordinates score normally", guess: &game.LatLng{}, score: 5000},
		{name: "disconnected participant may guess", guess: &game.LatLng{}, score: 5000, disconnected: true},
		{name: "finished round rejects", guess: &game.LatLng{}, finished: true, want: mp.ErrRoundAlreadyFinished},
		{name: "guess before start rejects", guess: &game.LatLng{}, offset: -time.Nanosecond, want: game.ErrRoundNotStarted},
		{name: "no guess before start rejects", offset: -time.Nanosecond, want: game.ErrRoundNotStarted},
		{name: "NaN rejects", guess: &game.LatLng{Lat: math.NaN()}, want: game.ErrInvalidCoordinates},
		{name: "out of range rejects", guess: &game.LatLng{Lng: 181}, want: game.ErrInvalidCoordinates},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)
			h.round.Finished = tt.finished
			req := dto.NewMultiplayerRoundGuessRequest{
				RequestTime: h.now.Add(tt.offset),
				GameID:      h.game.ID,
				UserID:      h.userID,
				Guess:       tt.guess,
			}

			h.expectTransaction()
			lock := h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()
			fetchGame := h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()
			participants := h.participants()
			participants[0].Connected = !tt.disconnected
			members := h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(participants, nil).Once()
			fetchRound := h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).
				Return(h.round, nil).Once()
			mock.InOrder(lock, fetchGame, members, fetchRound)

			if tt.want == nil {
				coordinates := game.LatLng{}
				if tt.guess != nil {
					coordinates = *tt.guess
				}

				write := h.repo.On("NewMultiplayerRoundGuess", mock.Anything, dto.NewMultiplayerRoundGuessRequestDB{
					RequestTime: req.RequestTime, UserID: h.userID, RoundID: h.round.ID,
					Lat: coordinates.Lat, Lng: coordinates.Lng, Score: tt.score, Distance: tt.distance,
				}).Return(nil).Once()
				mock.InOrder(fetchRound, write)
			}

			require.ErrorIs(t, h.uc.NewRoundGuess(t.Context(), req), tt.want)

			if tt.want != nil {
				h.repo.AssertNotCalled(t, "NewMultiplayerRoundGuess", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestUsecase_NewRoundGuessFailures(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{
		"transaction",
		"lock",
		"game",
		"invalid game",
		"participants",
		"not a participant",
		"round",
		"write",
		"commit",
	} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)
			cause := errors.New("repository unavailable")
			setupMultiplayerGuessFailure(h, failure, cause)

			want := cause
			if failure == "invalid game" {
				want = game.ErrInvalidPlayers
			}

			if failure == "not a participant" {
				want = mp.ErrGameWrongUserID
			}

			require.ErrorIs(t, h.uc.NewRoundGuess(t.Context(), dto.NewMultiplayerRoundGuessRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.userID,
			}), want)
		})
	}
}

func TestUsecase_EndRound(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name            string
		offset          time.Duration
		timer, count    int
		finished, final bool
		want            error
	}{
		{name: "untimed round may end before start", offset: -time.Nanosecond},
		{
			name:   "before deadline rejects partial guesses",
			timer:  30,
			offset: 30*time.Second - time.Nanosecond,
			count:  1,
			want:   mp.ErrRoundIsStillActive,
		},
		{name: "at deadline ends partial guesses", timer: 30, offset: 30 * time.Second, count: 1},
		{name: "after deadline ends partial guesses", timer: 30, offset: 30*time.Second + time.Nanosecond, count: 1},
		{name: "all players guessed ends early", timer: 30, count: 2},
		{name: "excess guesses reject", timer: 30, count: 3, want: game.ErrInvalidState},
		{name: "negative guesses reject", count: -1, want: game.ErrInvalidState},
		{name: "final round also ends game", final: true, timer: 30, offset: 30 * time.Second},
		{name: "finished final round returns guesses before timer check", finished: true, final: true, timer: 30},
		{name: "finished nonfinal round returns guesses", finished: true, timer: 30},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)

			h.game.TimerSeconds = tt.timer

			if tt.final {
				h.game.RoundCurrent = h.game.Rounds
			}

			h.round.RoundNum, h.round.Finished, h.round.GuessesCount = h.game.RoundCurrent, tt.finished, tt.count
			req := dto.EndMultiplayerRoundRequest{RequestTime: h.now.Add(tt.offset), GameID: h.game.ID, UserID: h.userID}
			guesses := []mp.Guess{
				{Username: "alice", RoundNum: h.round.RoundNum, Score: 3032},
				{Username: "bob", RoundNum: h.round.RoundNum, Score: 1234},
			}

			h.expectTransaction()
			lock := h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()
			members := h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(h.participants(), nil).Once()
			fetchGame := h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()
			fetchRound := h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).
				Return(h.round, nil).Once()
			fetchGuesses := h.repo.On("GetMultiplayerRoundGuesses", mock.Anything, h.round.ID).Return(guesses, nil).Once()
			mock.InOrder(lock, members, fetchGame, fetchRound, fetchGuesses)

			if !tt.finished && tt.want == nil {
				endRound := h.repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
					RequestTime: req.RequestTime,
					RoundID:     h.round.ID,
				}).
					Return(nil).Once()
				mock.InOrder(fetchGuesses, endRound)

				if tt.final {
					endGame := h.repo.On("EndMultiplayerGame", mock.Anything, dto.EndMultiplayerGameRequestDB{
						RequestTime: req.RequestTime,
						GameID:      h.game.ID,
					}).
						Return(nil).Once()
					mock.InOrder(endRound, endGame)
				}
			}

			got, err := h.uc.EndRound(t.Context(), req)
			require.ErrorIs(t, err, tt.want)

			if tt.want != nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, guesses, got)
			}

			if tt.finished || tt.want != nil {
				h.repo.AssertNotCalled(t, "EndMultiplayerRound", mock.Anything, mock.Anything)
				h.repo.AssertNotCalled(t, "EndMultiplayerGame", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestUsecase_EndRoundFailures(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{
		"transaction",
		"lock",
		"participants",
		"not a participant",
		"game",
		"invalid game",
		"round",
		"guesses",
		"end round",
		"end game",
		"commit",
	} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			h := newMultiplayerHarness(t)
			h.game.RoundCurrent, h.round.RoundNum = h.game.Rounds, h.game.Rounds
			cause := errors.New("repository unavailable")
			setupMultiplayerEndRoundFailure(h, failure, cause)

			want := cause
			if failure == "not a participant" {
				want = mp.ErrGameWrongUserID
			}

			if failure == "invalid game" {
				want = game.ErrInvalidPlayers
			}

			got, err := h.uc.EndRound(t.Context(), dto.EndMultiplayerRoundRequest{
				RequestTime: h.now,
				GameID:      h.game.ID,
				UserID:      h.userID,
			})
			require.ErrorIs(t, err, want)
			assert.Nil(t, got)
		})
	}
}

func setupMultiplayerGuessFailure(h *multiplayerHarness, failure string, cause error) {
	if !h.expectFailureTransaction(failure, cause) {
		return
	}

	if failure == "lock" {
		h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(cause).Once()
		return
	}

	h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()

	if failure == "game" {
		h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(mp.Game{}, cause).Once()
		return
	}

	if failure == "invalid game" {
		h.game.Players = 1
	}

	h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once()

	if failure == "invalid game" {
		return
	}

	if failure == "participants" {
		h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(nil, cause).Once()
		return
	}

	if failure == "not a participant" {
		h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return([]user.MultiplayerUser{}, nil).Once()
		return
	}

	h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).Return(h.participants(), nil).Once()

	if failure == "round" {
		h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(mp.Round{}, cause).Once()
		return
	}

	h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(h.round, nil).Once()

	var writeErr error
	if failure == "write" {
		writeErr = cause
	}

	h.repo.On("NewMultiplayerRoundGuess", mock.Anything, dto.NewMultiplayerRoundGuessRequestDB{
		RequestTime: h.now, UserID: h.userID, RoundID: h.round.ID,
	}).Return(writeErr).Once()
}

func setupMultiplayerEndRoundFailure(h *multiplayerHarness, failure string, cause error) {
	if !setupLockedMultiplayerGame(h, failure, cause) {
		return
	}

	if failure == "round" {
		h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(mp.Round{}, cause).Once()
		return
	}

	h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).Return(h.round, nil).Once()

	if failure == "guesses" {
		h.repo.On("GetMultiplayerRoundGuesses", mock.Anything, h.round.ID).Return(nil, cause).Once()
		return
	}

	h.repo.On("GetMultiplayerRoundGuesses", mock.Anything, h.round.ID).
		Return([]mp.Guess{{Username: "alice", Score: 5000}}, nil).Once()

	if failure == "end round" {
		h.repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
			RequestTime: h.now,
			RoundID:     h.round.ID,
		}).
			Return(cause).Once()

		return
	}

	endRound := h.repo.On("EndMultiplayerRound", mock.Anything, dto.EndMultiplayerRoundRequestDB{
		RequestTime: h.now,
		RoundID:     h.round.ID,
	}).
		Return(nil).Once()

	var endErr error
	if failure == "end game" {
		endErr = cause
	}

	endGame := h.repo.On("EndMultiplayerGame", mock.Anything, dto.EndMultiplayerGameRequestDB{
		RequestTime: h.now,
		GameID:      h.game.ID,
	}).
		Return(endErr).Once()
	mock.InOrder(endRound, endGame)
}

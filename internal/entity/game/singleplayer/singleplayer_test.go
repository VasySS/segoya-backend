package singleplayer_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
)

func TestGame_HasUser(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tests := []struct {
		name    string
		ownerID uuid.UUID
		userID  uuid.UUID
		wantErr error
	}{
		{name: "owner", ownerID: ownerID, userID: ownerID},
		{name: "another user", ownerID: ownerID, userID: uuid.Nil, wantErr: singleplayer.ErrGameWrongUserID},
		{name: "equal zero identifiers retain ownership semantics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := singleplayer.Game{UserID: tt.ownerID}
			assert.ErrorIs(t, g.HasUser(tt.userID), tt.wantErr)
		})
	}
}

func TestGame_ValidateNewRound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int
		rounds  int
		wantErr error
	}{
		{name: "first round", rounds: 5},
		{name: "below limit", current: 4, rounds: 5},
		{name: "at limit", current: 5, rounds: 5, wantErr: singleplayer.ErrRoundMaxAmount},
		{name: "above limit rejects invalid state", current: 6, rounds: 5, wantErr: game.ErrInvalidState},
		{name: "zero round count rejected", wantErr: game.ErrInvalidRounds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := singleplayer.Game{Provider: game.GoogleProvider, RoundCurrent: tt.current, Rounds: tt.rounds}
			assert.ErrorIs(t, g.ValidateNewRound(), tt.wantErr)
		})
	}
}

func TestGame_ValidateEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  int
		rounds   int
		finished bool
		wantErr  error
	}{
		{name: "all rounds created", current: 5, rounds: 5},
		{name: "missing rounds", current: 4, rounds: 5, wantErr: singleplayer.ErrGameIsStillActive},
		{name: "above count", current: 6, rounds: 5, wantErr: game.ErrInvalidState},
		{name: "zero counts rejected", wantErr: game.ErrInvalidRounds},
		{
			name: "finished game does not bypass count", current: 4, rounds: 5, finished: true,
			wantErr: game.ErrInvalidState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := singleplayer.Game{
				Provider:     game.GoogleProvider,
				RoundCurrent: tt.current,
				Rounds:       tt.rounds,
				Finished:     tt.finished,
			}
			assert.ErrorIs(t, g.ValidateEnd(), tt.wantErr)
		})
	}
}

func TestGame_ValidateGuessesAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		finished bool
		wantErr  error
	}{
		{name: "finished", finished: true},
		{name: "active", wantErr: singleplayer.ErrGameIsStillActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := singleplayer.Game{Provider: game.GoogleProvider, Rounds: 1, RoundCurrent: 1, Finished: tt.finished}
			assert.ErrorIs(t, g.ValidateGuessesAccess(), tt.wantErr)
		})
	}
}

func TestGame_PrepareNextRound(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	gameID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	panorama := game.PanoramaMetadata{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333")}
	tests := []struct {
		name    string
		current int
		delay   time.Duration
	}{
		{name: "first round"},
		{name: "later round with delay", current: 2, delay: 3500 * time.Millisecond},
		{name: "negative delay preserves arithmetic", current: 1, delay: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := singleplayer.Game{Provider: game.GoogleProvider, Rounds: 5, ID: gameID, RoundCurrent: tt.current}
			original := g
			got, err := g.PrepareNextRound(panorama, now, tt.delay)
			require.NoError(t, err)
			assert.Equal(t, game.RoundCreation{
				GameID: gameID, LocationID: panorama.ID, RoundNum: tt.current + 1,
				CreatedAt: now, StartedAt: now.Add(tt.delay),
			}, got)
			assert.Equal(t, original, g)
		})
	}
}

func TestRound_CanReuse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		finished bool
		want     bool
	}{
		{name: "unfinished", want: true},
		{name: "finished", finished: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			round := singleplayer.Round{Finished: tt.finished}
			assert.Equal(t, tt.want, round.CanReuse())
		})
	}
}

func TestRound_ValidateFinished(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		finished bool
		wantErr  error
	}{
		{name: "finished", finished: true},
		{name: "unfinished", wantErr: singleplayer.ErrRoundIsStillActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			round := singleplayer.Round{Finished: tt.finished}
			assert.ErrorIs(t, round.ValidateFinished(), tt.wantErr)
		})
	}
}

func TestRound_EvaluateGuess(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		provider     game.PanoramaProvider
		location     game.LatLng
		guess        game.LatLng
		elapsed      time.Duration
		timerSeconds int
		finished     bool
		wantScore    int
		wantDistance int
		wantErr      error
	}{
		{
			name: "before timer deadline", provider: game.GoogleProvider, guess: game.LatLng{Lat: 6.74637},
			elapsed: time.Minute - time.Nanosecond, timerSeconds: 60, wantScore: 3032, wantDistance: 750_000,
		},
		{
			name: "at timer deadline", provider: game.GoogleProvider, guess: game.LatLng{Lat: 6.74637},
			elapsed: time.Minute, timerSeconds: 60, wantScore: 3032, wantDistance: 750_000,
		},
		{
			name: "late guess keeps distance", provider: game.GoogleProvider, guess: game.LatLng{Lat: 6.74637},
			elapsed: time.Minute + time.Nanosecond, timerSeconds: 60, wantDistance: 750_000,
		},
		{
			name: "disabled timer", provider: game.GoogleProvider, guess: game.LatLng{Lat: 6.74637},
			elapsed: time.Hour, wantScore: 3032, wantDistance: 750_000,
		},
		{
			name: "yandex scoring", provider: game.YandexProvider, guess: game.LatLng{Lat: 4.49758},
			wantScore: 3032, wantDistance: 500_000,
		},
		{
			name: "seznam scoring", provider: game.SeznamProvider, guess: game.LatLng{Lat: 1.34928},
			wantScore: 3032, wantDistance: 150_000,
		},
		{
			name: "yandex air scoring", provider: game.YandexAirProvider, guess: game.LatLng{Lat: 6.74637},
			wantScore: 3032, wantDistance: 750_000,
		},
		{
			name: "exact nonzero location", provider: game.GoogleProvider,
			location: game.LatLng{Lat: 11.22, Lng: 22.33}, guess: game.LatLng{Lat: 11.22, Lng: 22.33},
			wantScore: 5000,
		},
		{name: "perfect zero guess", provider: game.GoogleProvider, wantScore: 5000},
		{
			name: "finished round rejects guess", provider: game.GoogleProvider, guess: game.LatLng{Lat: 6.74637},
			finished: true, wantErr: singleplayer.ErrRoundAlreadyFinished,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := singleplayer.Game{Provider: tt.provider, TimerSeconds: tt.timerSeconds}
			round := singleplayer.Round{
				Lat: tt.location.Lat, Lng: tt.location.Lng, StartedAt: startedAt, Finished: tt.finished,
			}
			score, distance, err := round.EvaluateGuess(g, startedAt.Add(tt.elapsed), &tt.guess)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantScore, score)
			assert.Equal(t, tt.wantDistance, distance)
		})
	}
}

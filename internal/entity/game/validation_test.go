package game_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestSettingsBoundaries(t *testing.T) {
	t.Parallel()

	for _, rounds := range []int{-1, 0, 1, 10, 11} {
		for _, timer := range []int{-1, 0, 1, 9, 10, 600, 601} {
			t.Run(fmt.Sprintf("rounds=%d/timer=%d", rounds, timer), func(t *testing.T) {
				t.Parallel()

				err := game.ValidateSettings(game.GoogleProvider, rounds, timer)

				switch {
				case rounds < 1 || rounds > 10:
					require.ErrorIs(t, err, game.ErrInvalidRounds)
				case timer != 0 && (timer < 10 || timer > 600):
					require.ErrorIs(t, err, game.ErrInvalidTimerSeconds)
				default:
					require.NoError(t, err)
				}
			})
		}
	}

	t.Run("provider error takes precedence", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, game.ValidateSettings("unknown", 0, -1), game.ErrInvalidProvider)
	})
}

func TestGuessBoundaries(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	for _, guess := range []struct {
		name            string
		coordinates     *game.LatLng
		score, distance int
	}{
		{name: "no guess"},
		{name: "zero coordinates", coordinates: &game.LatLng{}, score: 5000},
		{name: "north and east limits", coordinates: &game.LatLng{Lat: 90, Lng: 180}, distance: 10005389},
		{name: "south and west limits", coordinates: &game.LatLng{Lat: -90, Lng: -180}, distance: 10005389},
	} {
		t.Run(guess.name, func(t *testing.T) {
			t.Parallel()

			sp, mp := singleplayer.Round{StartedAt: start}, multiplayer.Round{StartedAt: start}
			settings := singleplayer.Game{Provider: game.GoogleProvider, TimerSeconds: 10}

			t.Run("before start", func(t *testing.T) {
				t.Parallel()

				_, _, err := sp.EvaluateGuess(settings, start.Add(-time.Nanosecond), guess.coordinates)
				require.ErrorIs(t, err, game.ErrRoundNotStarted)
				_, _, err = mp.EvaluateGuess(game.GoogleProvider, start.Add(-time.Nanosecond), guess.coordinates)
				require.ErrorIs(t, err, game.ErrRoundNotStarted)
			})

			for _, elapsed := range []time.Duration{
				0,
				time.Nanosecond,
				10*time.Second - time.Nanosecond,
				10 * time.Second,
				10*time.Second + time.Nanosecond,
			} {
				t.Run("elapsed="+elapsed.String(), func(t *testing.T) {
					t.Parallel()

					spScore, spDistance, err := sp.EvaluateGuess(settings, start.Add(elapsed), guess.coordinates)
					require.NoError(t, err)
					mpScore, mpDistance, err := mp.EvaluateGuess(game.GoogleProvider, start.Add(elapsed), guess.coordinates)
					require.NoError(t, err)

					wantSPScore := guess.score
					if elapsed > 10*time.Second {
						wantSPScore = 0
					}

					assert.Equal(t, wantSPScore, spScore)
					assert.Equal(t, guess.score, mpScore)
					assert.Equal(t, guess.distance, spDistance)
					assert.Equal(t, guess.distance, mpDistance)
				})
			}
		})
	}
}

func TestGuessInvalidCoordinates(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		coordinates game.LatLng
	}{
		{"NaN latitude", game.LatLng{Lat: math.NaN()}},
		{"NaN longitude", game.LatLng{Lng: math.NaN()}},
		{"infinite latitude", game.LatLng{Lat: math.Inf(1)}},
		{"infinite longitude", game.LatLng{Lng: math.Inf(-1)}},
		{"latitude too high", game.LatLng{Lat: 90.01}},
		{"latitude too low", game.LatLng{Lat: -90.01}},
		{"longitude too high", game.LatLng{Lng: 180.01}},
		{"longitude too low", game.LatLng{Lng: -180.01}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			settings := singleplayer.Game{Provider: game.GoogleProvider}
			_, _, err := (singleplayer.Round{}).EvaluateGuess(settings, time.Time{}, &tt.coordinates)
			require.ErrorIs(t, err, game.ErrInvalidCoordinates)
			_, _, err = (multiplayer.Round{}).EvaluateGuess(game.GoogleProvider, time.Time{}, &tt.coordinates)
			require.ErrorIs(t, err, game.ErrInvalidCoordinates)
		})
	}
}

func TestInvalidRoundPreparation(t *testing.T) {
	t.Parallel()

	for _, state := range []struct {
		name            string
		rounds, current int
		want            error
	}{
		{"zero rounds", 0, 0, game.ErrInvalidRounds},
		{"negative current round", 5, -1, game.ErrInvalidState},
		{"excess current round", 5, 6, game.ErrInvalidState},
		{"maximum rounds", 5, 5, singleplayer.ErrRoundMaxAmount},
	} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()

			sp := singleplayer.Game{Provider: game.GoogleProvider, Rounds: state.rounds, RoundCurrent: state.current}
			mp := multiplayer.Game{Provider: game.GoogleProvider, Players: 2, Rounds: state.rounds, RoundCurrent: state.current}
			_, err := sp.PrepareNextRound(game.PanoramaMetadata{}, time.Time{}, 0)
			require.ErrorIs(t, err, state.want)
			_, err = mp.PrepareNextRound(game.PanoramaMetadata{}, time.Time{}, 0)

			wantMP := state.want
			if state.current == state.rounds && state.rounds > 0 {
				wantMP = multiplayer.ErrRoundMaxAmount
			}

			require.ErrorIs(t, err, wantMP)
		})
	}

	require.False(t, multiplayer.Game{Finished: true}.IsComplete())
	require.False(t, singleplayer.Game{}.AllRoundsCreated())
}

func TestParticipantsValidation(t *testing.T) {
	t.Parallel()

	creator := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")
	second := uuid.MustParse("019a553a-fc00-7000-8000-000000000002")
	players := []user.PublicProfile{{ID: creator}, {ID: second}}

	maximum := append([]user.PublicProfile{}, players...)
	for i := byte(3); len(maximum) < 11; i++ {
		id := creator
		id[15] = i
		maximum = append(maximum, user.PublicProfile{ID: id})
	}

	for _, tt := range []struct {
		name    string
		creator uuid.UUID
		players []user.PublicProfile
		want    error
	}{
		{"minimum players", creator, players, nil},
		{"one player", creator, players[:1], game.ErrInvalidPlayers},
		{"creator missing", uuid.Nil, players, game.ErrInvalidPlayers},
		{"duplicate player", creator, []user.PublicProfile{players[0], players[0]}, game.ErrInvalidPlayers},
		{"maximum players", creator, maximum[:10], nil},
		{"too many players", creator, maximum, game.ErrInvalidPlayers},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, multiplayer.ValidateParticipants(tt.creator, tt.players), tt.want)
		})
	}
}

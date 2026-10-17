package game_test

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
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
			err := game.ValidateSettings(game.GoogleProvider, rounds, timer)

			switch {
			case rounds < 1 || rounds > 10:
				require.ErrorIs(t, err, game.ErrInvalidRounds)
			case timer != 0 && (timer < 10 || timer > 600):
				require.ErrorIs(t, err, game.ErrInvalidTimerSeconds)
			default:
				require.NoError(t, err)
			}
		}
	}

	require.ErrorIs(t, game.ValidateSettings("unknown", 0, -1), game.ErrInvalidProvider)
}

func TestGuessBoundaries(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	sp := singleplayer.Round{StartedAt: start}
	mp := multiplayer.Round{StartedAt: start}

	settings := singleplayer.Game{Provider: game.GoogleProvider, TimerSeconds: 10}
	for _, guess := range []*game.LatLng{nil, {}, {Lat: 90, Lng: 180}, {Lat: -90, Lng: -180}} {
		_, _, err := sp.EvaluateGuess(settings, start.Add(-time.Nanosecond), guess)
		require.ErrorIs(t, err, game.ErrRoundNotStarted)
		_, _, err = mp.EvaluateGuess(game.GoogleProvider, start.Add(-time.Nanosecond), guess)
		require.ErrorIs(t, err, game.ErrRoundNotStarted)

		for _, elapsed := range []time.Duration{
			0,
			time.Nanosecond,
			10*time.Second - time.Nanosecond,
			10 * time.Second,
			10*time.Second + time.Nanosecond,
		} {
			spScore, spDistance, spErr := sp.EvaluateGuess(settings, start.Add(elapsed), guess)
			mpScore, mpDistance, mpErr := mp.EvaluateGuess(game.GoogleProvider, start.Add(elapsed), guess)

			require.NoError(t, spErr)
			require.NoError(t, mpErr)
			require.Equal(t, spDistance, mpDistance)

			if guess == nil {
				require.Zero(t, spScore)
				require.Zero(t, mpScore)
				require.Zero(t, spDistance)
			} else if *guess == (game.LatLng{}) {
				require.Equal(t, 5000, mpScore)

				if elapsed > 10*time.Second {
					require.Zero(t, spScore)
				} else {
					require.Equal(t, 5000, spScore)
				}
			}
		}
	}

	for _, guess := range []game.LatLng{
		{Lat: math.NaN()},
		{Lng: math.NaN()},
		{Lat: math.Inf(1)},
		{Lng: math.Inf(-1)},
		{Lat: 90.01},
		{Lat: -90.01},
		{Lng: 180.01},
		{Lng: -180.01},
	} {
		_, _, err := sp.EvaluateGuess(settings, start, &guess)
		require.ErrorIs(t, err, game.ErrInvalidCoordinates)
		_, _, err = mp.EvaluateGuess(game.GoogleProvider, start, &guess)
		require.ErrorIs(t, err, game.ErrInvalidCoordinates)
	}
}

func TestInvalidRoundPreparation(t *testing.T) {
	t.Parallel()

	for _, state := range []struct{ rounds, current int }{{0, 0}, {5, -1}, {5, 6}, {5, 5}} {
		sp := singleplayer.Game{Provider: game.GoogleProvider, Rounds: state.rounds, RoundCurrent: state.current}
		mp := multiplayer.Game{Provider: game.GoogleProvider, Players: 2, Rounds: state.rounds, RoundCurrent: state.current}
		_, err := sp.PrepareNextRound(game.PanoramaMetadata{}, time.Now(), 0)
		require.Error(t, err)
		_, err = mp.PrepareNextRound(game.PanoramaMetadata{}, time.Now(), 0)
		require.Error(t, err)
	}

	require.False(t, multiplayer.Game{Finished: true}.IsComplete())
	require.False(t, singleplayer.Game{}.AllRoundsCreated())
}

func TestParticipantsValidation(t *testing.T) {
	t.Parallel()

	creator := uuid.New()
	players := []user.PublicProfile{{ID: creator}, {ID: uuid.New()}}
	require.NoError(t, multiplayer.ValidateParticipants(creator, players))
	require.ErrorIs(t, multiplayer.ValidateParticipants(creator, players[:1]), game.ErrInvalidPlayers)
	require.ErrorIs(t, multiplayer.ValidateParticipants(uuid.New(), players), game.ErrInvalidPlayers)
	require.ErrorIs(t, multiplayer.ValidateParticipants(creator, []user.PublicProfile{
		players[0],
		players[0],
	}), game.ErrInvalidPlayers)

	for len(players) < 10 {
		players = append(players, user.PublicProfile{ID: uuid.New()})
	}

	require.NoError(t, multiplayer.ValidateParticipants(creator, players))
	players = append(players, user.PublicProfile{ID: uuid.New()})
	require.ErrorIs(t, multiplayer.ValidateParticipants(creator, players), game.ErrInvalidPlayers)
}

package multiplayer_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestGame_RoundPredicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		game          multiplayer.Game
		wantMaxRounds bool
		wantComplete  bool
	}{
		{name: "zero game"},
		{name: "finished zero game", game: multiplayer.Game{Finished: true}, wantComplete: true},
		{name: "no rounds created", game: multiplayer.Game{Rounds: 5}},
		{name: "rounds remain", game: multiplayer.Game{Rounds: 5, RoundCurrent: 4, Finished: true}},
		{name: "last round active", game: multiplayer.Game{Rounds: 5, RoundCurrent: 5}, wantMaxRounds: true},
		{
			name: "last round finished", game: multiplayer.Game{Rounds: 5, RoundCurrent: 5, Finished: true},
			wantMaxRounds: true, wantComplete: true,
		},
		{name: "excess rounds active", game: multiplayer.Game{Rounds: 5, RoundCurrent: 6}},
		{
			name: "excess rounds finished", game: multiplayer.Game{Rounds: 5, RoundCurrent: 6, Finished: true},
			wantComplete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantMaxRounds, tt.game.MaxRoundsReached())
			assert.Equal(t, tt.wantComplete, tt.game.IsComplete())
		})
	}
}

func TestGame_IsFinalRound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rounds      int
		roundNumber int
		want        bool
	}{
		{name: "zero rounds", want: true},
		{name: "round remains", rounds: 5, roundNumber: 4},
		{name: "final round", rounds: 5, roundNumber: 5, want: true},
		{name: "excess round", rounds: 5, roundNumber: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			currentGame := multiplayer.Game{Rounds: tt.rounds}
			assert.Equal(t, tt.want, currentGame.IsFinalRound(tt.roundNumber))
		})
	}
}

func TestGame_PrepareNextRound(t *testing.T) {
	t.Parallel()

	requestTime := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	currentGame := multiplayer.Game{ID: uuid.New(), RoundCurrent: 2}
	panorama := game.PanoramaMetadata{ID: uuid.New()}

	assert.Equal(t, game.RoundCreation{
		GameID:     currentGame.ID,
		LocationID: panorama.ID,
		RoundNum:   3,
		CreatedAt:  requestTime,
		StartedAt:  requestTime.Add(5 * time.Second),
	}, currentGame.PrepareNextRound(panorama, requestTime, 5*time.Second))
}

func TestParticipants_HasUser(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tests := []struct {
		name         string
		participants multiplayer.Participants
		wantErr      error
	}{
		{name: "empty participants", wantErr: multiplayer.ErrGameWrongUserID},
		{name: "different participant", participants: multiplayer.Participants{Users: []user.MultiplayerUser{
			{PublicProfile: user.PublicProfile{ID: uuid.New()}, Connected: true},
		}}, wantErr: multiplayer.ErrGameWrongUserID},
		{name: "connected participant", participants: multiplayer.Participants{Users: []user.MultiplayerUser{
			{PublicProfile: user.PublicProfile{ID: userID}, Connected: true},
		}}},
		{name: "disconnected participant", participants: multiplayer.Participants{Users: []user.MultiplayerUser{
			{PublicProfile: user.PublicProfile{ID: userID}},
		}}},
		{name: "participant found later", participants: multiplayer.Participants{Users: []user.MultiplayerUser{
			{PublicProfile: user.PublicProfile{ID: uuid.New()}},
			{PublicProfile: user.PublicProfile{ID: userID}},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.participants.HasUser(userID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestRound_CanReuse(t *testing.T) {
	t.Parallel()

	endedAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	boundary := endedAt.Add(10 * time.Second)
	tests := []struct {
		name        string
		finished    bool
		requestTime time.Time
		want        bool
	}{
		{name: "unfinished before delay", requestTime: boundary.Add(-time.Nanosecond), want: true},
		{name: "unfinished at delay", requestTime: boundary, want: true},
		{name: "unfinished after delay", requestTime: boundary.Add(time.Nanosecond), want: true},
		{name: "finished before delay", finished: true, requestTime: boundary.Add(-time.Nanosecond), want: true},
		{name: "finished at delay", finished: true, requestTime: boundary},
		{name: "finished after delay", finished: true, requestTime: boundary.Add(time.Nanosecond)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			round := multiplayer.Round{Finished: tt.finished, EndedAt: endedAt}
			assert.Equal(t, tt.want, round.CanReuse(tt.requestTime, 10*time.Second))
		})
	}
}

func TestRound_EvaluateGuess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		round        multiplayer.Round
		guess        game.LatLng
		wantScore    int
		wantDistance int
		wantErr      error
	}{
		{name: "perfect guess", round: multiplayer.Round{Lat: 1}, guess: game.LatLng{Lat: 1}, wantScore: 5000},
		{name: "zero coordinate guess"},
		{name: "distant guess", guess: game.LatLng{Lat: 6.74637}, wantScore: 3032, wantDistance: 750000},
		{
			name: "finished round rejects", round: multiplayer.Round{Finished: true},
			guess: game.LatLng{Lat: 6.74637}, wantErr: multiplayer.ErrRoundAlreadyFinished,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			score, distance, err := tt.round.EvaluateGuess(game.GoogleProvider, tt.guess)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantScore, score)
			assert.Equal(t, tt.wantDistance, distance)
		})
	}
}

func TestRound_TimerPassed(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	boundary := startedAt.Add(30 * time.Second)
	tests := []struct {
		name         string
		requestTime  time.Time
		timerSeconds int
		want         bool
	}{
		{name: "no timer before start", requestTime: startedAt.Add(-time.Nanosecond), want: true},
		{name: "before timer", requestTime: boundary.Add(-time.Nanosecond), timerSeconds: 30},
		{name: "at timer", requestTime: boundary, timerSeconds: 30, want: true},
		{name: "after timer", requestTime: boundary.Add(time.Nanosecond), timerSeconds: 30, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			round := multiplayer.Round{StartedAt: startedAt}
			assert.Equal(t, tt.want, round.TimerPassed(tt.requestTime, tt.timerSeconds))
		})
	}
}

func TestRound_AllPlayersGuessed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		players      int
		guessesCount int
		want         bool
	}{
		{name: "no players", want: true},
		{name: "not all players guessed", players: 2, guessesCount: 1},
		{name: "all players guessed", players: 2, guessesCount: 2, want: true},
		{name: "extra guesses", players: 2, guessesCount: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			round := multiplayer.Round{GuessesCount: tt.guessesCount}
			assert.Equal(t, tt.want, round.AllPlayersGuessed(tt.players))
		})
	}
}

func TestRound_ValidateEnd(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	boundary := startedAt.Add(30 * time.Second)
	tests := []struct {
		name         string
		requestTime  time.Time
		timerSeconds int
		players      int
		guessesCount int
		wantErr      error
	}{
		{name: "no timer ends before start", requestTime: startedAt.Add(-time.Nanosecond), players: 2},
		{
			name: "before timer rejects", requestTime: boundary.Add(-time.Nanosecond), timerSeconds: 30,
			players: 2, guessesCount: 1, wantErr: multiplayer.ErrRoundIsStillActive,
		},
		{name: "at timer ends", requestTime: boundary, timerSeconds: 30, players: 2, guessesCount: 1},
		{name: "after timer ends", requestTime: boundary.Add(time.Nanosecond), timerSeconds: 30, players: 2},
		{name: "all guessed ends early", requestTime: startedAt, timerSeconds: 30, players: 2, guessesCount: 2},
		{name: "zero players ends early", requestTime: startedAt, timerSeconds: 30},
		{
			name: "extra guesses reject early", requestTime: startedAt, timerSeconds: 30, players: 2,
			guessesCount: 3, wantErr: multiplayer.ErrRoundIsStillActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			round := multiplayer.Round{StartedAt: startedAt, GuessesCount: tt.guessesCount}

			err := round.ValidateEnd(tt.requestTime, tt.timerSeconds, tt.players)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

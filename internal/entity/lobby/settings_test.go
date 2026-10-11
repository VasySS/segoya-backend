package lobby_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

func TestSettings_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings lobby.Settings
		wantErr  error
	}{
		{
			name:     "unsupported provider takes precedence",
			settings: lobby.Settings{Provider: "unknown", Rounds: 0, TimerSeconds: 9},
			wantErr:  lobby.ErrInvalidProvider,
		},
		{
			name: "empty provider", settings: lobby.Settings{Rounds: 1, TimerSeconds: 10},
			wantErr: lobby.ErrInvalidProvider,
		},
		{
			name: "provider is case sensitive", settings: lobby.Settings{Provider: "Google", Rounds: 1},
			wantErr: lobby.ErrInvalidProvider,
		},
		{
			name:     "rounds below minimum take precedence",
			settings: lobby.Settings{Provider: "google", Rounds: 0, TimerSeconds: 9},
			wantErr:  lobby.ErrInvalidRounds,
		},
		{
			name: "rounds above maximum", settings: lobby.Settings{Provider: "google", Rounds: 11},
			wantErr: lobby.ErrInvalidRounds,
		},
		{
			name: "negative timer", settings: lobby.Settings{Provider: "google", Rounds: 1, TimerSeconds: -1},
			wantErr: lobby.ErrInvalidTimerSeconds,
		},
		{
			name: "timer above untimed", settings: lobby.Settings{Provider: "google", Rounds: 1, TimerSeconds: 1},
			wantErr: lobby.ErrInvalidTimerSeconds,
		},
		{
			name: "timer below minimum", settings: lobby.Settings{Provider: "google", Rounds: 1, TimerSeconds: 9},
			wantErr: lobby.ErrInvalidTimerSeconds,
		},
		{
			name: "timer above maximum", settings: lobby.Settings{Provider: "google", Rounds: 1, TimerSeconds: 601},
			wantErr: lobby.ErrInvalidTimerSeconds,
		},
		{
			name: "minimum rounds and timer", settings: lobby.Settings{Provider: "google", Rounds: 1, TimerSeconds: 10},
		},
		{
			name: "maximum rounds and timer", settings: lobby.Settings{Provider: "yandex", Rounds: 10, TimerSeconds: 600},
		},
		{
			name: "untimed", settings: lobby.Settings{Provider: "yandex_air", Rounds: 5, TimerSeconds: 0},
		},
		{
			name: "seznam", settings: lobby.Settings{Provider: "seznam", Rounds: 5, TimerSeconds: 60},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, tt.settings.Validate(), tt.wantErr)
		})
	}
}

func TestSettings_JSON(t *testing.T) {
	t.Parallel()

	settings := lobby.Settings{Provider: "google", MovementAllowed: true, Rounds: 5, TimerSeconds: 60}
	payload, err := json.Marshal(settings)
	require.NoError(t, err)
	assert.JSONEq(t, `{"provider":"google","movementAllowed":true,"rounds":5,"timerSeconds":60}`, string(payload))

	var decoded lobby.Settings

	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, settings, decoded)
}

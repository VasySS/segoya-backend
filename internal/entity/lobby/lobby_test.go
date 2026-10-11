package lobby_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/lobby"
)

func TestLobby_IsFull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int
		maximum int
		want    bool
	}{
		{name: "below capacity", current: 4, maximum: 5},
		{name: "at capacity", current: 5, maximum: 5, want: true},
		{name: "above capacity", current: 6, maximum: 5, want: true},
		{name: "zero capacity", current: 0, maximum: 0, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value := lobby.Lobby{CurrentPlayers: tt.current, MaxPlayers: tt.maximum}
			assert.Equal(t, tt.want, value.IsFull())
		})
	}
}

func TestLobby_IsLastPlayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		players int
		want    bool
	}{
		{name: "negative player count", players: -1},
		{name: "empty lobby", players: 0},
		{name: "last player", players: 1, want: true},
		{name: "other players remain", players: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value := lobby.Lobby{CurrentPlayers: tt.players}
			assert.Equal(t, tt.want, value.IsLastPlayer())
		})
	}
}

func TestLobby_ValidateStart(t *testing.T) {
	t.Parallel()

	creatorID := uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6943")
	value := lobby.Lobby{CreatorID: creatorID}
	tests := []struct {
		name    string
		actorID uuid.UUID
		wantErr error
	}{
		{name: "creator", actorID: creatorID},
		{
			name: "other user", actorID: uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6944"),
			wantErr: lobby.ErrOnlyCreatorCanStart,
		},
		{name: "missing actor", wantErr: lobby.ErrOnlyCreatorCanStart},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, value.ValidateStart(tt.actorID), tt.wantErr)
		})
	}
}

func TestLobby_WithSettings(t *testing.T) {
	t.Parallel()

	creatorID := uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6943")
	original := lobby.Lobby{
		ID: "test-lobby", CreatorID: creatorID, CreatedAt: time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC),
		Provider: "yandex", Rounds: 5, TimerSeconds: 30,
		CurrentPlayers: 2, MaxPlayers: 5, Private: true,
	}
	tests := []struct {
		name     string
		actorID  uuid.UUID
		settings lobby.Settings
		want     lobby.Lobby
		wantErr  error
	}{
		{
			name: "creator updates a copy and preserves metadata", actorID: creatorID,
			settings: lobby.Settings{Provider: "google", MovementAllowed: true, Rounds: 10, TimerSeconds: 60},
			want: lobby.Lobby{
				ID: "test-lobby", CreatorID: creatorID, CreatedAt: original.CreatedAt,
				Provider: "google", MovementAllowed: true, Rounds: 10, TimerSeconds: 60,
				CurrentPlayers: 2, MaxPlayers: 5, Private: true,
			},
		},
		{
			name: "identical settings", actorID: creatorID,
			settings: lobby.Settings{Provider: "yandex", Rounds: 5, TimerSeconds: 30}, want: original,
		},
		{
			name: "validation remains an entrypoint responsibility", actorID: creatorID,
			settings: lobby.Settings{Provider: "unknown", Rounds: 0, TimerSeconds: 9},
			want: lobby.Lobby{
				ID: "test-lobby", CreatorID: creatorID, CreatedAt: original.CreatedAt,
				Provider: "unknown", Rounds: 0, TimerSeconds: 9,
				CurrentPlayers: 2, MaxPlayers: 5, Private: true,
			},
		},
		{
			name: "noncreator cannot change settings", actorID: uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6944"),
			settings: lobby.Settings{Provider: "google", MovementAllowed: true, Rounds: 10, TimerSeconds: 60},
			wantErr:  lobby.ErrOnlyCreatorCanUpdateSettings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value := original
			got, err := value.WithSettings(tt.actorID, tt.settings)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, original, value)
		})
	}
}

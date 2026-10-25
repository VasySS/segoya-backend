package valkey //nolint:testpackage // Exercise the internal persisted-data parser without a running Valkey instance.

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

func TestParseLobbyPlayArea(t *testing.T) {
	t.Parallel()

	polygon := playarea.Selection{Type: "polygon", Polygon: &playarea.Polygon{
		Type: "Polygon", Coordinates: [][][]float64{{{10, 20}, {11, 20}, {11, 21}, {10, 20}}},
	}}
	encoded, err := json.Marshal(polygon)
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		data string
		want playarea.Selection
		err  bool
	}{
		{name: "legacy missing selection", want: playarea.Selection{Type: "all"}},
		{name: "stored polygon", data: string(encoded), want: polygon},
		{name: "malformed JSON", data: "{", err: true},
		{name: "invalid selection", data: `{"type":"presets"}`, err: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := map[string]string{
				lobbyCreatorIDField: uuid.NewString(), lobbyCreatedAtField: "2026-10-25T00:00:00Z",
				lobbyRoundsField: "3", lobbyTimerSecondsField: "60", lobbyMovementAllowedField: "true",
				lobbyCurrentPlayersField: "2", lobbyMaxPlayersField: "5", lobbyPrivateField: "false",
				lobbyProviderField: "google", lobbyPlayAreaField: tc.data,
			}

			got, err := parseLobbyData("test", data)
			if tc.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got.PlayArea)
		})
	}
}

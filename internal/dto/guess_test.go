package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
)

func TestGuessPresenceContract(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, payload string }{
		{"guess omitted", `{}`},
		{"coordinates omitted", `{"guess":{}}`},
		{"longitude omitted", `{"guess":{"lat":0}}`},
		{"latitude omitted", `{"guess":{"lng":0}}`},
		{"null latitude", `{"guess":{"lat":null,"lng":0}}`},
		{"null longitude", `{"guess":{"lat":0,"lng":null}}`},
		{"string latitude", `{"guess":{"lat":"NaN","lng":0}}`},
		{"overflowing latitude", `{"guess":{"lat":1e1000,"lng":0}}`},
		{"boolean guess", `{"guess":false}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var mp dto.MultiplayerUserGuessMessage

			require.Error(t, json.Unmarshal([]byte(tt.payload), &mp))

			var sp api.SingleplayerRoundGuess

			require.Error(t, json.Unmarshal([]byte(tt.payload), &sp))
		})
	}

	for _, tt := range []struct {
		name, payload string
		want          *game.LatLng
	}{
		{"explicit no guess", `{"guess":null}`, nil},
		{"zero coordinates", `{"guess":{"lat":0,"lng":0}}`, &game.LatLng{}},
		{"maximum coordinates", `{"guess":{"lat":90,"lng":180}}`, &game.LatLng{Lat: 90, Lng: 180}},
		{"minimum coordinates", `{"guess":{"lat":-90,"lng":-180}}`, &game.LatLng{Lat: -90, Lng: -180}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var mp dto.MultiplayerUserGuessMessage

			require.NoError(t, json.Unmarshal([]byte(tt.payload), &mp))
			assert.Equal(t, tt.want, mp.Guess)

			var sp api.SingleplayerRoundGuess

			require.NoError(t, json.Unmarshal([]byte(tt.payload), &sp))
			require.NoError(t, sp.Validate())
			assert.Equal(t, tt.want, dto.LatLngAPIToEntity(sp.Guess))
		})
	}

	for _, tt := range []struct{ name, payload string }{
		{"latitude too high", `{"guess":{"lat":91,"lng":0}}`},
		{"latitude too low", `{"guess":{"lat":-91,"lng":0}}`},
		{"longitude too high", `{"guess":{"lat":0,"lng":181}}`},
		{"longitude too low", `{"guess":{"lat":0,"lng":-181}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var mp dto.MultiplayerUserGuessMessage

			require.ErrorIs(t, json.Unmarshal([]byte(tt.payload), &mp), game.ErrInvalidCoordinates)

			var sp api.SingleplayerRoundGuess

			require.NoError(t, json.Unmarshal([]byte(tt.payload), &sp))
			require.Error(t, sp.Validate())
		})
	}
}

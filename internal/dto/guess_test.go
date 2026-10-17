package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
)

func TestGuessPresenceContract(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{}`,
		`{"guess":{}}`,
		`{"guess":{"lat":0}}`,
		`{"guess":{"lng":0}}`,
		`{"guess":{"lat":null,"lng":0}}`,
		`{"guess":{"lat":0,"lng":null}}`,
		`{"guess":{"lat":"NaN","lng":0}}`,
		`{"guess":{"lat":1e1000,"lng":0}}`,
		`{"guess":false}`,
	} {
		var mp dto.MultiplayerUserGuessMessage

		require.Error(t, json.Unmarshal([]byte(payload), &mp), payload)

		var sp api.SingleplayerRoundGuess

		require.Error(t, json.Unmarshal([]byte(payload), &sp), payload)
	}

	for _, payload := range []string{
		`{"guess":null}`,
		`{"guess":{"lat":0,"lng":0}}`,
		`{"guess":{"lat":90,"lng":180}}`,
		`{"guess":{"lat":-90,"lng":-180}}`,
	} {
		var mp dto.MultiplayerUserGuessMessage

		require.NoError(t, json.Unmarshal([]byte(payload), &mp))

		var sp api.SingleplayerRoundGuess

		require.NoError(t, json.Unmarshal([]byte(payload), &sp))
		require.NoError(t, sp.Validate())
		require.Equal(t, mp.Guess, dto.LatLngAPIToEntity(sp.Guess))
	}

	for _, payload := range []string{`{"guess":{"lat":91,"lng":0}}`, `{"guess":{"lat":0,"lng":181}}`} {
		var mp dto.MultiplayerUserGuessMessage

		require.Error(t, json.Unmarshal([]byte(payload), &mp))

		var sp api.SingleplayerRoundGuess

		require.NoError(t, json.Unmarshal([]byte(payload), &sp))
		require.Error(t, sp.Validate())
	}
}

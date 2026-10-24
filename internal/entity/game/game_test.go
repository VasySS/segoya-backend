package game_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

func TestLatLng_DistanceTo(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		from, to   game.LatLng
		wantMeters float64
		tolerance  float64
	}{
		{name: "same point", from: game.LatLng{Lat: 45, Lng: 30}, to: game.LatLng{Lat: 45, Lng: 30}},
		{name: "one latitude degree", to: game.LatLng{Lat: 1}, wantMeters: 111176, tolerance: 50},
		{
			name:       "antimeridian takes short path",
			from:       game.LatLng{Lng: 179.9},
			to:         game.LatLng{Lng: -179.9},
			wantMeters: 22235,
			tolerance:  20,
		},
		{
			name:      "longitudes meet at pole",
			from:      game.LatLng{Lat: 90, Lng: -120},
			to:        game.LatLng{Lat: 90, Lng: 80},
			tolerance: 1e-6,
		},
		{name: "opposite poles", from: game.LatLng{Lat: -90}, to: game.LatLng{Lat: 90}, wantMeters: 20011000, tolerance: 500},
		{
			name:       "nearly antipodal remains finite",
			from:       game.LatLng{Lat: 0.000001},
			to:         game.LatLng{Lat: -0.000001, Lng: 179.999999},
			wantMeters: 20011000,
			tolerance:  500,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			distance := tt.from.DistanceTo(tt.to)
			require.False(t, math.IsNaN(distance) || math.IsInf(distance, 0))
			assert.GreaterOrEqual(t, distance, 0.0)
			assert.InDelta(t, tt.wantMeters, distance, tt.tolerance)
			assert.InDelta(t, distance, tt.to.DistanceTo(tt.from), 1e-6)
		})
	}
}

func FuzzDistanceAndScore(f *testing.F) {
	for _, coordinates := range [][4]float64{
		{0, 0, 0, 0},
		{12.3, -45.6, 12.3, -45.6},
		{0, 179.9, 0, -179.9},
		{-90, -180, 90, 180},
		{0.000001, 0, -0.000001, 179.999999},
	} {
		f.Add(coordinates[0], coordinates[1], coordinates[2], coordinates[3])
	}

	f.Fuzz(func(t *testing.T, lat1, lng1, lat2, lng2 float64) {
		from, to := game.LatLng{Lat: lat1, Lng: lng1}, game.LatLng{Lat: lat2, Lng: lng2}
		if from.Validate() != nil || to.Validate() != nil {
			t.Skip()
		}

		distance := from.DistanceTo(to)
		require.False(t, math.IsNaN(distance) || math.IsInf(distance, 0))
		require.InDelta(t, distance, to.DistanceTo(from), 1e-6)
		require.InDelta(t, 0, from.DistanceTo(from), 1e-6)
		require.GreaterOrEqual(t, distance, 0.0)
		require.LessOrEqual(t, distance, 20_020_000.0)

		for _, provider := range []game.PanoramaProvider{
			game.GoogleProvider,
			game.YandexProvider,
			game.YandexAirProvider,
			game.SeznamProvider,
		} {
			score, meters := game.CalculateScore(provider, from, to)
			require.GreaterOrEqual(t, score, 0, "provider %s", provider)
			require.LessOrEqual(t, score, 5000, "provider %s", provider)
			require.Equal(t, int(distance), meters)

			selfScore, selfDistance := game.CalculateScore(provider, from, from)
			require.Equal(t, 5000, selfScore)
			require.Zero(t, selfDistance)
		}
	})
}

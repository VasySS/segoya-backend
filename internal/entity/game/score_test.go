package game_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

func TestCalculateScore(t *testing.T) {
	t.Parallel()

	type args struct {
		provider game.PanoramaProvider
		realLat  float64
		realLng  float64
		userLat  float64
		userLng  float64
	}

	tests := []struct {
		name         string
		args         args
		wantScore    int
		wantDistance int
	}{
		{
			name: "full score",
			args: args{
				provider: "google",
				realLat:  11.22,
				realLng:  22.33,
				userLat:  11.22,
				userLng:  22.33,
			},
			wantScore:    5000,
			wantDistance: 0,
		},
		{
			name: "~60% score (yandex)",
			args: args{
				provider: "yandex",
				realLat:  0.0,
				realLng:  0.0,
				userLat:  4.49758,
				userLng:  0.0,
			},
			wantScore:    3032,
			wantDistance: 500_000,
		},
		{
			name: "~60% score (seznam)",
			args: args{
				provider: "seznam",
				realLat:  0.0,
				realLng:  0.0,
				userLat:  1.34928,
				userLng:  0.0,
			},
			wantScore:    3032,
			wantDistance: 150_000,
		},
		{
			name: "~60% score (google)",
			args: args{
				provider: "google",
				realLat:  0.0,
				realLng:  0.0,
				userLat:  6.74637,
				userLng:  0.0,
			},
			wantScore:    3032,
			wantDistance: 750_000,
		},
		{
			name: "zero coordinates scored normally",
			args: args{
				provider: "google",
				realLat:  11.11,
				realLng:  22.22,
				userLat:  0.0,
				userLng:  0.0,
			},
			wantScore:    6,
			wantDistance: 2747733,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			location := game.LatLng{Lat: tt.args.realLat, Lng: tt.args.realLng}
			guess := game.LatLng{Lat: tt.args.userLat, Lng: tt.args.userLng}
			score, distance := game.CalculateScore(tt.args.provider, location, guess)

			assert.Equal(t, tt.wantScore, score)
			assert.Equal(t, tt.wantDistance, distance)
		})
	}
}

func TestAreaScoringCurve(t *testing.T) {
	t.Parallel()

	degrees := 20_000 / game.EarthRadiusMeters * 180 / math.Pi
	scoring, err := game.AreaScoring(game.GeographicBounds{MinLng: -degrees / 2, MaxLng: degrees / 2})
	require.NoError(t, err)
	require.NoError(t, scoring.Validate())
	require.Equal(t, game.ScoringVersionArea, scoring.Version)

	for _, tc := range []struct {
		name   string
		meters float64
		points int
	}{
		{name: "perfect", points: 5000},
		{name: "100 metres", meters: 100, points: 4889},
		{name: "1 kilometre", meters: 1000, points: 4000},
		{name: "2 kilometres", meters: 2000, points: 3200},
		{name: "5 kilometres", meters: 5000, points: 1638},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			guess := game.LatLng{Lat: tc.meters / game.EarthRadiusMeters * 180 / math.Pi}
			points, distance := scoring.Score(game.LatLng{}, guess)
			require.InDelta(t, tc.points, points, 1)
			require.InDelta(t, tc.meters, distance, 1)
		})
	}

	previous := 5000

	for lat := 0.0; lat <= 180; lat++ {
		points, _ := scoring.Score(game.LatLng{}, game.LatLng{Lng: lat})
		require.GreaterOrEqual(t, points, 0)
		require.LessOrEqual(t, points, previous)
		previous = points
	}

	require.Zero(t, previous)
}

func TestAreaScoringExtent(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		bounds       game.GeographicBounds
		wantDiameter float64
	}{
		{name: "tiny area", bounds: game.GeographicBounds{MaxLng: 0.0001}, wantDiameter: 1000},
		{name: "hemisphere cap", bounds: game.GeographicBounds{MaxLng: 360}, wantDiameter: math.Pi * game.EarthRadiusMeters},
		{
			name: "equatorial degree", bounds: game.GeographicBounds{MaxLng: 1},
			wantDiameter: game.EarthRadiusMeters * math.Pi / 180,
		},
		{
			name: "high latitude", bounds: game.GeographicBounds{MinLat: 60, MaxLat: 60, MaxLng: 1},
			wantDiameter: game.EarthRadiusMeters * math.Pi / 360,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scoring, err := game.AreaScoring(tc.bounds)
			require.NoError(t, err)
			require.InDelta(t, tc.wantDiameter, scoring.ScaleMeters*20*math.Log(1.25), 0.001)
		})
	}
}

func TestScoringRejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	for _, settings := range []game.ScoringSettings{
		{},
		{Version: 3, ScaleMeters: 1000},
		{Version: 1, ScaleMeters: -1},
		{Version: 2, ScaleMeters: math.NaN()},
		{Version: 2, ScaleMeters: math.Inf(1)},
	} {
		require.ErrorIs(t, settings.Validate(), game.ErrInvalidScoring)
	}

	for _, bounds := range []game.GeographicBounds{
		{MinLng: 1},
		{MaxLat: 91},
		{MinLat: -91},
		{MinLat: 1},
		{MaxLng: 361},
		{MaxLng: math.NaN()},
		{MinLat: math.Inf(1)},
	} {
		_, err := game.AreaScoring(bounds)
		require.ErrorIs(t, err, game.ErrInvalidScoring)
	}
}

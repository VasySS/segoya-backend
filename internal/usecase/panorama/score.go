package panorama

import (
	"math"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// CalculateScoreAndDistance returns score (0 to 5000) and distance in meters for provided coordinates.
// Score is adjusted depending on provider.
// Based on this:
// https://stackoverflow.com/questions/65351282
func (uc Usecase) CalculateScoreAndDistance(
	provider game.PanoramaProvider,
	realLat, realLng, userLat, userLng float64,
) (int, int) {
	if userLat == 0.0 && userLng == 0.0 {
		return 0, 0
	}

	// distance in km, at which player will receive ~60% of score
	var scoreModifier float64

	switch provider {
	case game.SeznamProvider:
		scoreModifier = 150.0
	case game.YandexProvider:
		scoreModifier = 500.0
	case game.GoogleProvider, game.YandexAirProvider:
		scoreModifier = 750.0
	}

	distance := distanceMeters(realLat, realLng, userLat, userLng)
	score := 5000 * math.Exp(-0.5*math.Pow(((distance/1000)/scoreModifier), 2)) //nolint:staticcheck

	return int(score), int(distance)
}

// distanceMeters returns distance between two points in meters, based on this:
// https://stackoverflow.com/questions/8832071
func distanceMeters(latA, lngA, latB, lngB float64) float64 {
	earthRadius := 3958.75
	latDiff := latB - latA
	lngDiff := lngB - lngA
	latDiffRad := latDiff * (math.Pi / 180.0)
	lngDiffRad := lngDiff * (math.Pi / 180.0)

	a := math.Sin(latDiffRad/2)*math.Sin(latDiffRad/2) +
		math.Cos(latA*(math.Pi/180.0))*math.Cos(latB*(math.Pi/180.0))*
			math.Sin(lngDiffRad/2)*math.Sin(lngDiffRad/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	meterConversion := 1609.0

	return float64(distance * meterConversion)
}

package game

import "math"

const maxScore = 5000

// CalculateScore returns score (0 to 5000) and distance in meters for provided coordinates.
// Score is adjusted depending on provider.
func CalculateScore(provider PanoramaProvider, location, guess LatLng) (int, int) {
	distance := location.DistanceTo(guess)
	score := maxScore * math.Exp(-0.5*math.Pow(((distance/1000)/provider.ScoreModifier()), 2))

	return int(score), int(distance)
}

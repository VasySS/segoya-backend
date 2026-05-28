package game

import "math"

const maxScore = 5000

// CalculateScore returns score (0 to 5000) and distance in meters for provided coordinates.
// Score is adjusted depending on provider.
func CalculateScore(provider PanoramaProvider, real, guess LatLng) (int, int) {
	if guess.Lat == 0.0 && guess.Lng == 0.0 {
		return 0, 0
	}

	distance := real.DistanceTo(guess)
	score := maxScore * math.Exp(-0.5*math.Pow(((distance/1000)/provider.ScoreModifier()), 2))

	return int(score), int(distance)
}

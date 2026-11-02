package game

import (
	"errors"
	"math"
)

const (
	maxScore               = 5000
	ScoringVersionProvider = 1
	ScoringVersionArea     = 2
)

// ErrInvalidScoring rejects unknown scoring versions, invalid bounds, and nonfinite scales.
var ErrInvalidScoring = errors.New("invalid scoring settings")

// ScoringSettings freezes a game's curve and distance scale in metres.
type ScoringSettings struct {
	Version     int     `json:"version"`
	ScaleMeters float64 `json:"scaleMeters"`
}

// GeographicBounds contains an ordinary or longitude-shifted WGS84 bounding box.
type GeographicBounds struct {
	MinLng float64
	MinLat float64
	MaxLng float64
	MaxLat float64
}

// ProviderScoring retains the original provider-dependent Gaussian curve.
func ProviderScoring(provider PanoramaProvider) ScoringSettings {
	return ScoringSettings{Version: ScoringVersionProvider, ScaleMeters: provider.ScoreModifier() * 1000}
}

// AreaScoring derives the exponential scale from the selected boundary's extent.
func AreaScoring(bounds GeographicBounds) (ScoringSettings, error) {
	values := []float64{bounds.MinLng, bounds.MinLat, bounds.MaxLng, bounds.MaxLat}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ScoringSettings{}, ErrInvalidScoring
		}
	}

	longitudeSpan := bounds.MaxLng - bounds.MinLng

	latitudeSpan := bounds.MaxLat - bounds.MinLat

	if longitudeSpan < 0 || longitudeSpan > 360 || latitudeSpan < 0 || bounds.MinLat < -90 || bounds.MaxLat > 90 {
		return ScoringSettings{}, ErrInvalidScoring
	}

	toRadians := math.Pi / 180
	midLatitude := (bounds.MinLat + bounds.MaxLat) / 2 * toRadians
	height := EarthRadiusMeters * latitudeSpan * toRadians
	width := EarthRadiusMeters * longitudeSpan * toRadians * math.Cos(midLatitude)
	diameter := math.Max(1000, math.Min(math.Pi*EarthRadiusMeters, math.Hypot(width, height)))

	return ScoringSettings{Version: ScoringVersionArea, ScaleMeters: diameter / (20 * math.Log(1.25))}, nil
}

// Validate checks persisted settings before scoring a guess.
func (s ScoringSettings) Validate() error {
	known := s.Version == ScoringVersionProvider || s.Version == ScoringVersionArea
	if !known || s.ScaleMeters <= 0 || math.IsNaN(s.ScaleMeters) || math.IsInf(s.ScaleMeters, 0) {
		return ErrInvalidScoring
	}

	return nil
}

// Score returns points and actual distance, preserving integer truncation.
// Callers must validate settings and coordinates before evaluating a guess.
func (s ScoringSettings) Score(location, guess LatLng) (int, int) {
	distance := location.DistanceTo(guess)
	normalized := distance / s.ScaleMeters

	exponent := -normalized

	if s.Version == ScoringVersionProvider {
		exponent = -0.5 * normalized * normalized
	}

	return int(maxScore * math.Exp(exponent)), int(distance)
}

// CalculateScore returns score (0 to 5000) and distance in meters for provided coordinates.
// Score is adjusted depending on provider.
func CalculateScore(provider PanoramaProvider, location, guess LatLng) (int, int) {
	return ProviderScoring(provider).Score(location, guess)
}

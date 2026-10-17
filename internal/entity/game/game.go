// Package game contains types for working with game data.
package game

import (
	"math"
)

// LatLng is a struct that contains latitude and longitude coordinates.
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Validate rejects non-finite coordinates and values outside geographic bounds.
func (l LatLng) Validate() error {
	if math.IsNaN(l.Lat) || math.IsInf(l.Lat, 0) || l.Lat < -90 || l.Lat > 90 ||
		math.IsNaN(l.Lng) || math.IsInf(l.Lng, 0) || l.Lng < -180 || l.Lng > 180 {
		return ErrInvalidCoordinates
	}

	return nil
}

// DistanceTo returns the Haversine distance in meters between two LatLng points.
func (l LatLng) DistanceTo(other LatLng) float64 {
	earthRadius := 3958.75
	latDiff := other.Lat - l.Lat
	lngDiff := other.Lng - l.Lng
	latDiffRad := latDiff * (math.Pi / 180.0)
	lngDiffRad := lngDiff * (math.Pi / 180.0)

	a := math.Sin(latDiffRad/2)*math.Sin(latDiffRad/2) +
		math.Cos(l.Lat*(math.Pi/180.0))*math.Cos(other.Lat*(math.Pi/180.0))*
			math.Sin(lngDiffRad/2)*math.Sin(lngDiffRad/2)
	a = math.Max(0, math.Min(1, a))

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	meterConversion := 1609.0

	return distance * meterConversion
}

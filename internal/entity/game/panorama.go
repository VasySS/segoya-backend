package game

import "github.com/google/uuid"

// PanoramaProvider is a type of panorama provider (who is hosting streetview images).
type PanoramaProvider string

// Supported panorama providers.
const (
	GoogleProvider    PanoramaProvider = "google"
	YandexProvider    PanoramaProvider = "yandex"
	YandexAirProvider PanoramaProvider = "yandex_air"
	SeznamProvider    PanoramaProvider = "seznam"
)

// IsSupportedProvider returns true if the provider is supported.
func IsSupportedProvider(p string) bool {
	switch PanoramaProvider(p) {
	case GoogleProvider, YandexProvider, YandexAirProvider, SeznamProvider:
		return true
	default:
		return false
	}
}

// ScoreModifier returns the distance modifier used in score calculation for this provider.
func (p PanoramaProvider) ScoreModifier() float64 {
	switch p {
	case SeznamProvider:
		return 150.0
	case YandexProvider:
		return 500.0
	case GoogleProvider, YandexAirProvider:
		return 750.0
	default:
		return 0
	}
}

// PanoramaMetadata contains general streetview metadata.
type PanoramaMetadata struct {
	LatLng

	ID           uuid.UUID
	StreetviewID string
}

// GoogleStreetview contains Google streetview metadata.
type GoogleStreetview struct {
	ID  uuid.UUID
	Lat float64
	Lng float64
}

// SeznamStreetview contains Seznam streetview metadata.
type SeznamStreetview struct {
	ID  uuid.UUID
	Lat float64
	Lng float64
}

// YandexAirview contains Yandex air view metadata.
type YandexAirview struct {
	ID           uuid.UUID
	StreetviewID string
	Lat          float64
	Lng          float64
}

// YandexStreetview contains Yandex streetview metadata.
type YandexStreetview struct {
	ID  uuid.UUID
	Lat float64
	Lng float64
}

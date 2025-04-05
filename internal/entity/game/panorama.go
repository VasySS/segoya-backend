package game

// PanoramaProvider is a type of panorama provider (who is hosting streetview images).
type PanoramaProvider string

// Supported panorama providers.
const (
	GoogleProvider    PanoramaProvider = "google"
	YandexProvider    PanoramaProvider = "yandex"
	YandexAirProvider PanoramaProvider = "yandex_air"
	SeznamProvider    PanoramaProvider = "seznam"
)

// PanoramaMetadata contains general streetview metadata.
type PanoramaMetadata struct {
	LatLng
	ID           int
	StreetviewID string
}

// GoogleStreetview contains Google streetview metadata.
type GoogleStreetview struct {
	ID  int
	Lat float64
	Lng float64
}

// SeznamStreetview contains Seznam streetview metadata.
type SeznamStreetview struct {
	ID  int
	Lat float64
	Lng float64
}

// YandexAirview contains Yandex air view metadata.
type YandexAirview struct {
	ID           int
	StreetviewID string
	Lat          float64
	Lng          float64
}

// YandexStreetview contains Yandex streetview metadata.
type YandexStreetview struct {
	ID  int
	Lat float64
	Lng float64
}

package dto

import (
	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// LatLngAPIToEntity converts an API LatLng object to an entity LatLng object.
func LatLngAPIToEntity(latlng api.NilLatLng) *game.LatLng {
	if latlng.IsNull() {
		return nil
	}

	return &game.LatLng{
		Lat: latlng.Value.Lat,
		Lng: latlng.Value.Lng,
	}
}

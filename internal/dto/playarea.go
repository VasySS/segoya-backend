package dto

import (
	"encoding/json"
	"fmt"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// PlayAreaFromAPI validates an optional request selection; omission selects all locations.
func PlayAreaFromAPI(value api.OptPlayAreaSelection) (playarea.Selection, error) {
	v, ok := value.Get()
	if !ok {
		return playarea.Selection{}, nil
	}

	result := playarea.Selection{Type: string(v.Type), PresetIDs: v.PresetIDs}

	if polygon, set := v.Polygon.Get(); set {
		p := PolygonFromAPI(polygon)
		result.Polygon = &p
	}

	if id, set := v.SavedID.Get(); set {
		result.SavedID = id
	}

	normalized, err := result.Normalize()
	if err != nil {
		return playarea.Selection{}, fmt.Errorf("decode play area: %w", err)
	}

	return normalized, nil
}

func PolygonFromAPI(p api.Polygon) playarea.Polygon {
	return playarea.Polygon{Type: string(p.Type), Coordinates: p.Coordinates}
}

func PolygonToAPI(p playarea.Polygon) api.Polygon {
	return api.Polygon{Type: api.PolygonType(p.Type), Coordinates: p.Coordinates}
}

// PlayAreaToAPI exposes applied geometry, never future round locations.
func PlayAreaToAPI(s playarea.Selection) api.OptPlayAreaSelection {
	if s.Type == "" {
		return api.OptPlayAreaSelection{}
	}

	result := api.PlayAreaSelection{Type: api.PlayAreaSelectionType(s.Type), PresetIDs: s.PresetIDs}
	if s.Polygon != nil {
		result.Polygon.SetTo(PolygonToAPI(*s.Polygon))
	}

	if s.SavedID != [16]byte{} {
		result.SavedID.SetTo(s.SavedID)
	}

	return api.NewOptPlayAreaSelection(result)
}

func SavedPlayAreaToAPI(s playarea.Saved) *api.SavedPlayArea {
	return &api.SavedPlayArea{
		ID:         s.ID,
		OwnerID:    s.OwnerID,
		Name:       s.Name,
		Polygon:    PolygonToAPI(s.Polygon),
		Visibility: api.SavedPlayAreaVisibility(s.Visibility),
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

func PlayAreaPresetToAPI(p playarea.Preset, includeCount bool) (*api.PlayAreaPreset, error) {
	result := &api.PlayAreaPreset{
		ID:          p.ID,
		Name:        p.Name,
		Kind:        p.Kind,
		CountryCode: p.CountryCode,
		Version:     p.Version,
		Source:      p.Source,
		License:     p.License,
		Attribution: p.Attribution,
		Bounds:      p.Bounds,
		Providers:   make([]api.Provider, 0, len(p.Providers)),
	}
	for _, provider := range p.Providers {
		result.Providers = append(result.Providers, api.Provider(provider))
	}

	if includeCount {
		result.LocationCount.SetTo(p.LocationCount)
	}

	if len(p.Polygon) > 0 {
		var geometry api.PresetGeometry
		if err := json.Unmarshal(p.Polygon, &geometry); err != nil {
			return nil, fmt.Errorf("decode preset display geometry: %w", err)
		}

		result.Polygon.SetTo(geometry)
	}

	return result, nil
}

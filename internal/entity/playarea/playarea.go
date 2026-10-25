// Package playarea defines geographic selections independently of panorama providers.
package playarea

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxBodyBytes = 64 << 10
	MaxVertices  = 500
	MaxPresets   = 300
)

var (
	ErrInvalidSelection      = errors.New("invalid play area selection")
	ErrInvalidPolygon        = errors.New("invalid play area polygon")
	ErrNotFound              = errors.New("play area not found")
	ErrInsufficientLocations = errors.New("not enough distinct locations for the requested rounds")
	ErrInvalidName           = errors.New("play area name must contain 1 to 100 characters")
	ErrInvalidVisibility     = errors.New("play area visibility must be private or unlisted")
)

// Selection is a tagged union. A zero value means the entire provider's coverage.
type Selection struct {
	Type      string      `json:"type"`
	PresetIDs []uuid.UUID `json:"presetIDs,omitempty"`
	Polygon   *Polygon    `json:"polygon,omitempty"`
	SavedID   uuid.UUID   `json:"savedID,omitzero"`
}

// Polygon is a GeoJSON Polygon geometry, with coordinates in longitude/latitude order.
type Polygon struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

// Normalize validates the tagged selection and returns a detached canonical value.
func (s Selection) Normalize() (Selection, error) {
	if s.Type == "" {
		s.Type = "all"
	}

	if !s.validFields() {
		return Selection{}, ErrInvalidSelection
	}

	switch s.Type {
	case "presets":
		s.PresetIDs = slices.Clone(s.PresetIDs)
		slices.SortFunc(s.PresetIDs, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

		s.PresetIDs = slices.Compact(s.PresetIDs)
		if slices.Contains(s.PresetIDs, uuid.Nil) {
			return Selection{}, ErrInvalidSelection
		}
	case "polygon":
		if err := s.Polygon.Validate(); err != nil {
			return Selection{}, err
		}

		p := s.Polygon.Clone()
		s.Polygon = &p
	}

	return s, nil
}

func (s Selection) validFields() bool {
	var present uint8
	if len(s.PresetIDs) > 0 {
		present |= 1
	}

	if s.Polygon != nil {
		present |= 2
	}

	if s.SavedID != uuid.Nil {
		present |= 4
	}

	expected, known := map[string]uint8{"all": 0, "presets": 1, "polygon": 2, "saved": 4}[s.Type]

	return known && present == expected && len(s.PresetIDs) <= MaxPresets
}

// Clone makes geometry snapshots independent of a caller's mutable coordinate slices.
func (p Polygon) Clone() Polygon {
	result := Polygon{Type: p.Type, Coordinates: make([][][]float64, len(p.Coordinates))}
	for i, ring := range p.Coordinates {
		result.Coordinates[i] = make([][]float64, len(ring))
		for j, point := range ring {
			result.Coordinates[i][j] = slices.Clone(point)
		}
	}

	return result
}

// Validate rejects unsupported topology before geometry is sent to PostGIS.
func (p Polygon) Validate() error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidPolygon, reason) }
	if p.Type != "Polygon" || len(p.Coordinates) != 1 {
		return invalid("one Polygon ring without holes is required")
	}

	ring := p.Coordinates[0]
	if len(ring) < 4 || len(ring) > MaxVertices+1 {
		return invalid("a closed ring with 3 to 500 vertices is required")
	}

	if err := validateRingVertices(ring); err != nil {
		return err
	}

	return validateRingGeometry(ring)
}

func validateRingVertices(ring [][]float64) error {
	seen := make(map[[2]float64]bool, len(ring))

	for i, point := range ring {
		if !validPoint(point) {
			return fmt.Errorf("%w: expected finite [longitude, latitude] pairs", ErrInvalidPolygon)
		}

		if i == len(ring)-1 {
			break
		}

		key := [2]float64{point[0], point[1]}
		if seen[key] {
			return fmt.Errorf("%w: vertices must be distinct", ErrInvalidPolygon)
		}

		seen[key] = true
	}

	if !slices.Equal(ring[0], ring[len(ring)-1]) {
		return fmt.Errorf("%w: ring must be closed", ErrInvalidPolygon)
	}

	return nil
}

func validPoint(point []float64) bool {
	if len(point) != 2 {
		return false
	}

	return finiteWithin(point[0], -180, 180) && finiteWithin(point[1], -90, 90)
}

func finiteWithin(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func validateRingGeometry(ring [][]float64) error {
	var area float64

	for i := range len(ring) - 1 {
		a, b := ring[i], ring[i+1]
		if math.Abs(a[0]-b[0]) > 180 {
			return fmt.Errorf("%w: antimeridian crossings are not supported", ErrInvalidPolygon)
		}

		area += a[0]*b[1] - b[0]*a[1]

		for j := i + 1; j < len(ring)-1; j++ {
			if j == i+1 || (i == 0 && j == len(ring)-2) {
				continue
			}

			if intersects(a, b, ring[j], ring[j+1]) {
				return fmt.Errorf("%w: ring must not intersect itself", ErrInvalidPolygon)
			}
		}
	}

	if area == 0 {
		return fmt.Errorf("%w: polygon must have nonzero area", ErrInvalidPolygon)
	}

	return nil
}

func intersects(a, b, c, d []float64) bool {
	cross := func(p, q, r []float64) float64 { return (q[0]-p[0])*(r[1]-p[1]) - (q[1]-p[1])*(r[0]-p[0]) }
	x, y, z, w := cross(a, b, c), cross(a, b, d), cross(c, d, a), cross(c, d, b)

	return opposite(x, y) && opposite(z, w) ||
		onSegment(a, b, c, x) || onSegment(a, b, d, y) || onSegment(c, d, a, z) || onSegment(c, d, b, w)
}

func opposite(a, b float64) bool { return (a > 0 && b < 0) || (a < 0 && b > 0) }

func onSegment(a, b, p []float64, cross float64) bool {
	return cross == 0 && p[0] >= math.Min(a[0], b[0]) && p[0] <= math.Max(a[0], b[0]) &&
		p[1] >= math.Min(a[1], b[1]) && p[1] <= math.Max(a[1], b[1])
}

// Saved is an owner's reusable provider-independent custom polygon.
type Saved struct {
	ID         uuid.UUID `json:"id"`
	OwnerID    uuid.UUID `json:"ownerID"`
	Name       string    `json:"name"`
	Polygon    Polygon   `json:"polygon"`
	Visibility string    `json:"visibility"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Preset is an immutable boundary version. Display geometry is never used for counting.
type Preset struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Kind          string          `json:"kind"`
	CountryCode   string          `json:"countryCode"`
	Version       string          `json:"version"`
	Source        string          `json:"source"`
	License       string          `json:"license"`
	Attribution   string          `json:"attribution"`
	Providers     []string        `json:"providers"`
	Bounds        []float64       `json:"bounds"`
	Polygon       json.RawMessage `json:"polygon,omitempty"`
	LocationCount int             `json:"locationCount"`
}

// ValidateName checks the trimmed human-readable saved area name.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 100 {
		return ErrInvalidName
	}

	return nil
}

// ValidateVisibility accepts only private or link-shared areas.
func ValidateVisibility(visibility string) error {
	if visibility != "private" && visibility != "unlisted" {
		return ErrInvalidVisibility
	}

	return nil
}

// IsInvalid identifies area errors that should be returned as bad requests.
func IsInvalid(err error) bool {
	return errors.Is(err, ErrInvalidSelection) || errors.Is(err, ErrInvalidPolygon) ||
		errors.Is(err, ErrInsufficientLocations) || errors.Is(err, ErrInvalidName) || errors.Is(err, ErrInvalidVisibility)
}

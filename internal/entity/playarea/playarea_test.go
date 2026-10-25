package playarea_test

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

func polygon(points ...[]float64) playarea.Polygon {
	return playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{points}}
}

func TestPolygonValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		p     playarea.Polygon
		valid bool
	}{
		{"triangle", polygon([]float64{0, 0}, []float64{2, 0}, []float64{0, 2}, []float64{0, 0}), true},
		{"opposite winding", polygon([]float64{0, 0}, []float64{0, 2}, []float64{2, 0}, []float64{0, 0}), true},
		{"not closed", polygon([]float64{0, 0}, []float64{2, 0}, []float64{0, 2}, []float64{1, 1}), false},
		{
			"self intersection",
			polygon([]float64{0, 0}, []float64{2, 2}, []float64{0, 2}, []float64{2, 0}, []float64{0, 0}),
			false,
		},
		{"zero area", polygon([]float64{0, 0}, []float64{1, 1}, []float64{2, 2}, []float64{0, 0}), false},
		{"date line", polygon([]float64{179, 0}, []float64{-179, 0}, []float64{-179, 1}, []float64{179, 0}), false},
		{"missing ordinate", polygon([]float64{0, 0}, []float64{2}, []float64{0, 2}, []float64{0, 0}), false},
		{"out of range", polygon([]float64{0, 0}, []float64{181, 0}, []float64{0, 2}, []float64{0, 0}), false},
		{"non finite", polygon([]float64{0, 0}, []float64{math.NaN(), 0}, []float64{0, 2}, []float64{0, 0}), false},
		{
			"repeated vertex",
			polygon([]float64{0, 0}, []float64{2, 0}, []float64{2, 0}, []float64{0, 2}, []float64{0, 0}),
			false,
		},
		{"unsupported geometry", playarea.Polygon{Type: "MultiPolygon"}, false},
		{"holes", playarea.Polygon{Type: "Polygon", Coordinates: make([][][]float64, 2)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.p.Validate()
			if tt.valid {
				require.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, playarea.ErrInvalidPolygon)
			}
		})
	}
}

func TestSelectionNormalizationAndSnapshots(t *testing.T) {
	t.Parallel()

	all, err := (playarea.Selection{}).Normalize()
	require.NoError(t, err)
	assert.Equal(t, "all", all.Type)

	id := uuid.New()
	selected, err := (playarea.Selection{Type: "presets", PresetIDs: []uuid.UUID{id, id}}).Normalize()
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{id}, selected.PresetIDs)

	p := polygon([]float64{0, 0}, []float64{2, 0}, []float64{0, 2}, []float64{0, 0})
	snapshot, err := (playarea.Selection{Type: "polygon", Polygon: &p}).Normalize()
	require.NoError(t, err)

	p.Coordinates[0][1][0] = 100

	assert.InDelta(t, 2.0, snapshot.Polygon.Coordinates[0][1][0], 0)

	for _, s := range []playarea.Selection{
		{Type: "other"},
		{Type: "presets"},
		{Type: "presets", PresetIDs: []uuid.UUID{uuid.Nil}},
		{Type: "all", Polygon: &p},
		{Type: "saved"},
		{Type: "saved", SavedID: id, Polygon: &p},
		{Type: "polygon", PresetIDs: []uuid.UUID{id}, Polygon: &p},
	} {
		_, err := s.Normalize()
		assert.Error(t, err)
	}
}

func TestVertexLimit(t *testing.T) {
	t.Parallel()

	for _, n := range []int{500, 501} {
		ring := make([][]float64, n+1)

		for i := range n {
			angle := float64(i) * 2 * math.Pi / float64(n)
			ring[i] = []float64{math.Cos(angle), math.Sin(angle)}
		}

		ring[n] = []float64{1, 0}

		err := polygon(ring...).Validate()
		if n == 500 {
			require.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, playarea.ErrInvalidPolygon)
		}
	}
}

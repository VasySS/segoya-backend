package playarea_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	area "github.com/VasySS/segoya-backend/internal/entity/playarea"
	usecase "github.com/VasySS/segoya-backend/internal/usecase/playarea"
)

type scoringRepository struct {
	usecase.Repository

	bounds   game.GeographicBounds
	err      error
	deadline time.Time
}

func (r *scoringRepository) GetPlayAreaPresetBounds(
	ctx context.Context, _ game.PanoramaProvider, _ []uuid.UUID,
) (game.GeographicBounds, error) {
	r.deadline, _ = ctx.Deadline()
	return r.bounds, r.err
}

func TestResolveScoringUsesAppliedBoundary(t *testing.T) {
	t.Parallel()

	polygon := area.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{20, 10}, {21, 10}, {21, 11}, {20, 10}}}}
	repo := &scoringRepository{bounds: game.GeographicBounds{MinLng: 20, MinLat: 10, MaxLng: 21, MaxLat: 11}}
	uc := usecase.NewUsecase(repo)
	want, err := game.AreaScoring(repo.bounds)
	require.NoError(t, err)
	custom, err := uc.ResolveScoring(t.Context(), game.GoogleProvider, area.Selection{Type: "polygon", Polygon: &polygon})
	require.NoError(t, err)
	require.Equal(t, want, custom)
	preset, err := uc.ResolveScoring(t.Context(), game.GoogleProvider,
		area.Selection{Type: "presets", PresetIDs: []uuid.UUID{uuid.New()}})
	require.NoError(t, err)
	require.Equal(t, custom, preset)
	require.WithinDuration(t, time.Now().Add(3*time.Second), repo.deadline, time.Second)
	all, err := uc.ResolveScoring(t.Context(), game.GoogleProvider, area.Selection{})
	require.NoError(t, err)
	require.Equal(t, game.ProviderScoring(game.GoogleProvider), all)

	_, err = uc.ResolveScoring(t.Context(), game.GoogleProvider, area.Selection{Type: "saved", SavedID: uuid.New()})
	require.ErrorIs(t, err, area.ErrInvalidSelection, "applied snapshots must never reopen a saved library entry")
	_, err = uc.ResolveScoring(t.Context(), "invalid", area.Selection{})
	require.ErrorIs(t, err, game.ErrInvalidProvider)

	repo.err = errors.New("geometry query unavailable")
	_, err = uc.ResolveScoring(t.Context(), game.GoogleProvider,
		area.Selection{Type: "presets", PresetIDs: []uuid.UUID{uuid.New()}})
	require.ErrorIs(t, err, repo.err)
}

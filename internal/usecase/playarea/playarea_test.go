package playarea_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	area "github.com/VasySS/segoya-backend/internal/entity/playarea"
	usecase "github.com/VasySS/segoya-backend/internal/usecase/playarea"
)

type repository struct {
	usecase.Repository

	count    int
	saved    area.Saved
	seen     area.Selection
	user     uuid.UUID
	deadline time.Time
}

func (r *repository) GetPlayArea(_ context.Context, id, user uuid.UUID) (area.Saved, error) {
	r.user = user
	return r.saved, nil
}

func (r *repository) CountPlayAreaLocations(
	ctx context.Context,
	_ game.PanoramaProvider,
	s area.Selection,
) (int, error) {
	r.seen = s
	r.deadline, _ = ctx.Deadline()

	return r.count, nil
}

func (r *repository) CreatePlayArea(
	_ context.Context,
	owner uuid.UUID,
	name string,
	p area.Polygon,
	visibility string,
) (area.Saved, error) {
	return area.Saved{OwnerID: owner, Name: name, Polygon: p, Visibility: visibility}, nil
}

func TestResolveSavedAreaFreezesGeometry(t *testing.T) {
	t.Parallel()

	p := area.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {2, 0}, {0, 2}, {0, 0}}}}
	r := &repository{saved: area.Saved{Polygon: p}}
	uc := usecase.NewUsecase(r)
	owner := uuid.New()
	snapshot, err := uc.Resolve(t.Context(), game.GoogleProvider,
		area.Selection{Type: "saved", SavedID: uuid.New()}, owner)
	require.NoError(t, err)
	assert.Equal(t, owner, r.user)
	assert.Equal(t, "polygon", snapshot.Type)

	r.saved.Polygon.Coordinates[0][1][0] = 10

	assert.InDelta(t, 2.0, snapshot.Polygon.Coordinates[0][1][0], 0)
}

func TestCountAndPrepareCapacity(t *testing.T) {
	t.Parallel()

	r := &repository{}
	uc := usecase.NewUsecase(r)
	n, err := uc.Count(t.Context(), game.GoogleProvider, area.Selection{}, uuid.New())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, "all", r.seen.Type)
	assert.WithinDuration(t, time.Now().Add(3*time.Second), r.deadline, time.Second)
	_, err = uc.Prepare(t.Context(), game.GoogleProvider, area.Selection{}, uuid.New(), 5)
	require.ErrorIs(t, err, area.ErrInsufficientLocations)

	r.count = 5
	_, err = uc.Prepare(t.Context(), game.GoogleProvider, area.Selection{}, uuid.New(), 5)
	require.NoError(t, err)
	_, err = uc.Count(t.Context(), "invalid", area.Selection{}, uuid.New())
	assert.ErrorIs(t, err, game.ErrInvalidProvider)
}

func TestCreateDefaultsPrivate(t *testing.T) {
	t.Parallel()

	uc := usecase.NewUsecase(&repository{})
	p := area.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {2, 0}, {0, 2}, {0, 0}}}}
	saved, err := uc.Create(t.Context(), uuid.New(), "  Home  ", p, "")
	require.NoError(t, err)
	assert.Equal(t, "Home", saved.Name)
	assert.Equal(t, "private", saved.Visibility)
	_, err = uc.Create(t.Context(), uuid.New(), " ", p, "private")
	assert.ErrorIs(t, err, area.ErrInvalidName)
}

package panorama_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama/mocks"
)

type streetviewProvider struct {
	name         game.PanoramaProvider
	get, random  string
	metadata     any
	streetviewID string
}

func TestUsecase_StreetviewDispatch(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")
	for _, provider := range []streetviewProvider{
		{
			name: game.GoogleProvider, get: "GetGoogleStreetview", random: "RandomGoogleStreetview",
			metadata: game.GoogleStreetview{ID: id, Lat: 12.3, Lng: -45.6},
		},
		{
			name: game.YandexProvider, get: "GetYandexStreetview", random: "RandomYandexStreetview",
			metadata: game.YandexStreetview{ID: id, Lat: 12.3, Lng: -45.6},
		},
		{
			name: game.YandexAirProvider, get: "GetYandexAirview", random: "RandomYandexAirview",
			metadata:     game.YandexAirview{ID: id, Lat: 12.3, Lng: -45.6, StreetviewID: "airview-id"},
			streetviewID: "airview-id",
		},
		{
			name: game.SeznamProvider, get: "GetSeznamStreetview", random: "RandomSeznamStreetview",
			metadata: game.SeznamStreetview{ID: id, Lat: 12.3, Lng: -45.6},
		},
	} {
		for _, operation := range []string{"random", "by ID"} {
			for _, outcome := range []struct {
				name string
				err  error
			}{
				{name: "success"}, {name: "repository failure", err: errors.New("panorama unavailable")},
			} {
				t.Run(fmt.Sprintf("%s/%s/%s", provider.name, operation, outcome.name), func(t *testing.T) {
					t.Parallel()
					testStreetviewCase(t, provider, operation, id, outcome.err)
				})
			}
		}
	}
}

func testStreetviewCase(t *testing.T, provider streetviewProvider, operation string, id uuid.UUID, wantErr error) {
	t.Helper()
	repo := mocks.NewRepository(t)
	uc := panorama.NewUsecase(panorama.Config{}, repo)

	var (
		got game.PanoramaMetadata
		err error
	)

	if operation == "random" {
		repo.On(provider.random, mock.Anything).Return(provider.metadata, wantErr).Once()
		got, err = uc.NewStreetview(t.Context(), provider.name)
	} else {
		repo.On(provider.get, mock.Anything, id).Return(provider.metadata, wantErr).Once()
		got, err = uc.GetStreetview(t.Context(), provider.name, id)
	}

	require.ErrorIs(t, err, wantErr)

	if wantErr != nil {
		assert.Equal(t, game.PanoramaMetadata{}, got)
		return
	}

	assert.Equal(t, game.PanoramaMetadata{
		ID: id, LatLng: game.LatLng{Lat: 12.3, Lng: -45.6}, StreetviewID: provider.streetviewID,
	}, got)
}

func TestUsecase_UnsupportedStreetviewProvider(t *testing.T) {
	t.Parallel()

	for _, provider := range []game.PanoramaProvider{"", "unknown", "Google"} {
		t.Run("provider="+string(provider), func(t *testing.T) {
			t.Parallel()

			uc := panorama.NewUsecase(panorama.Config{}, nil)
			created, err := uc.NewStreetview(t.Context(), provider)
			require.ErrorIs(t, err, panorama.ErrUnknownProvider)
			assert.Equal(t, game.PanoramaMetadata{}, created)
			found, err := uc.GetStreetview(t.Context(), provider, uuid.Nil)
			require.ErrorIs(t, err, panorama.ErrUnknownProvider)
			assert.Equal(t, game.PanoramaMetadata{}, found)
		})
	}
}

package panorama_test

import (
	"errors"
	"testing"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUsecase_NewYandexAirview(t *testing.T) {
	t.Parallel()

	yandexMetadata := game.YandexAirview{
		ID:           432432,
		StreetviewID: "some_streetview_id",
		Lat:          1.1234,
		Lng:          2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID:           yandexMetadata.ID,
		StreetviewID: yandexMetadata.StreetviewID,
		LatLng: game.LatLng{
			Lat: yandexMetadata.Lat,
			Lng: yandexMetadata.Lng,
		},
	}

	type fields struct {
		repo *mocks.PanoramaRepository
	}

	type args struct{}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    game.PanoramaMetadata
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get yandex airview",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomYandexAirview", mock.Anything).
					Return(yandexMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting yandex airview from repository",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomYandexAirview", mock.Anything).
					Return(game.YandexAirview{}, errors.New("some repository error"))
			},
			want:    game.PanoramaMetadata{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewPanoramaRepository(t)
			fs := fields{repo: repo}
			uc := panorama.NewUsecase(panorama.Config{}, repo)

			tt.setup(fs, tt.args)

			got, err := uc.NewYandexAirview(t.Context())
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetYandexAirview(t *testing.T) {
	t.Parallel()

	yandexMetadata := game.YandexAirview{
		ID:           432432,
		StreetviewID: "some_streetview_id",
		Lat:          1.1234,
		Lng:          2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID:           yandexMetadata.ID,
		StreetviewID: yandexMetadata.StreetviewID,
		LatLng: game.LatLng{
			Lat: yandexMetadata.Lat,
			Lng: yandexMetadata.Lng,
		},
	}

	type fields struct {
		repo *mocks.PanoramaRepository
	}

	type args struct {
		id int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    game.PanoramaMetadata
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get seznam streetview",
			args: args{id: yandexMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetYandexAirview", mock.Anything, a.id).
					Return(yandexMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting seznam streetview from repository",
			args: args{id: yandexMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetYandexAirview", mock.Anything, a.id).
					Return(game.YandexAirview{}, errors.New("some repository error"))
			},
			want:    game.PanoramaMetadata{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewPanoramaRepository(t)
			fs := fields{repo: repo}
			uc := panorama.NewUsecase(panorama.Config{}, repo)

			tt.setup(fs, tt.args)

			got, err := uc.GetYandexAirview(t.Context(), tt.args.id)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_NewYandexStreetview(t *testing.T) {
	t.Parallel()

	yandexMetadata := game.YandexStreetview{
		ID:  432432,
		Lat: 1.1234,
		Lng: 2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID: yandexMetadata.ID,
		LatLng: game.LatLng{
			Lat: yandexMetadata.Lat,
			Lng: yandexMetadata.Lng,
		},
	}

	type fields struct {
		repo *mocks.PanoramaRepository
	}

	type args struct{}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    game.PanoramaMetadata
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get yandex streetview",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomYandexStreetview", mock.Anything).
					Return(yandexMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting yandex streetview from repository",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomYandexStreetview", mock.Anything).
					Return(game.YandexStreetview{}, errors.New("some repository error"))
			},
			want:    game.PanoramaMetadata{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewPanoramaRepository(t)
			fs := fields{repo: repo}
			uc := panorama.NewUsecase(panorama.Config{}, repo)

			tt.setup(fs, tt.args)

			got, err := uc.NewYandexStreetview(t.Context())
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetYandexStreetview(t *testing.T) {
	t.Parallel()

	yandexMetadata := game.YandexStreetview{
		ID:  432432,
		Lat: 1.1234,
		Lng: 2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID: yandexMetadata.ID,
		LatLng: game.LatLng{
			Lat: yandexMetadata.Lat,
			Lng: yandexMetadata.Lng,
		},
	}

	type fields struct {
		repo *mocks.PanoramaRepository
	}

	type args struct {
		id int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    game.PanoramaMetadata
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get yandex streetview",
			args: args{id: yandexMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetYandexStreetview", mock.Anything, a.id).
					Return(yandexMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting yandex streetview from repository",
			args: args{id: yandexMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetYandexStreetview", mock.Anything, a.id).
					Return(game.YandexStreetview{}, errors.New("some repository error"))
			},
			want:    game.PanoramaMetadata{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewPanoramaRepository(t)
			fs := fields{repo: repo}
			uc := panorama.NewUsecase(panorama.Config{}, repo)

			tt.setup(fs, tt.args)

			got, err := uc.GetYandexStreetview(t.Context(), tt.args.id)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

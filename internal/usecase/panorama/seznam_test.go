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

func TestUsecase_NewSeznamStreetview(t *testing.T) {
	t.Parallel()

	seznamMetadata := game.SeznamStreetview{
		ID:  432432,
		Lat: 1.1234,
		Lng: 2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID: seznamMetadata.ID,
		LatLng: game.LatLng{
			Lat: seznamMetadata.Lat,
			Lng: seznamMetadata.Lng,
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
			name: "successfully get seznam streetview",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomSeznamStreetview", mock.Anything).
					Return(seznamMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting google streetview from repository",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomSeznamStreetview", mock.Anything).
					Return(game.SeznamStreetview{}, errors.New("some repository error"))
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

			got, err := uc.NewSeznamStreetview(t.Context())
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetSeznamStreetview(t *testing.T) {
	t.Parallel()

	seznamMetadata := game.SeznamStreetview{
		ID:  432432,
		Lat: 1.1234,
		Lng: 2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID: seznamMetadata.ID,
		LatLng: game.LatLng{
			Lat: seznamMetadata.Lat,
			Lng: seznamMetadata.Lng,
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
			args: args{id: seznamMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetSeznamStreetview", mock.Anything, a.id).
					Return(seznamMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting seznam streetview from repository",
			args: args{id: seznamMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetSeznamStreetview", mock.Anything, a.id).
					Return(game.SeznamStreetview{}, errors.New("some repository error"))
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

			got, err := uc.GetSeznamStreetview(t.Context(), tt.args.id)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

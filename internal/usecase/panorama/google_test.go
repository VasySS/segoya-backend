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

func TestUsecase_NewGoogleStreetview(t *testing.T) {
	t.Parallel()

	googleMetadata := game.GoogleStreetview{
		ID:  432432,
		Lat: 1.1234,
		Lng: 2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID: googleMetadata.ID,
		LatLng: game.LatLng{
			Lat: googleMetadata.Lat,
			Lng: googleMetadata.Lng,
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
			name: "successfully get google streetview",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomGoogleStreetview", mock.Anything).
					Return(googleMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting google streetview from repository",
			args: args{},
			setup: func(f fields, _ args) {
				f.repo.On("RandomGoogleStreetview", mock.Anything).
					Return(game.GoogleStreetview{}, errors.New("some repository error"))
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

			got, err := uc.NewGoogleStreetview(t.Context())
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetGoogleStreetview(t *testing.T) {
	t.Parallel()

	googleMetadata := game.GoogleStreetview{
		ID:  432432,
		Lat: 1.1234,
		Lng: 2.3456,
	}

	panoMetadata := game.PanoramaMetadata{
		ID: googleMetadata.ID,
		LatLng: game.LatLng{
			Lat: googleMetadata.Lat,
			Lng: googleMetadata.Lng,
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
			name: "successfully get google streetview",
			args: args{id: googleMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetGoogleStreetview", mock.Anything, a.id).
					Return(googleMetadata, nil)
			},
			want:    panoMetadata,
			wantErr: assert.NoError,
		},
		{
			name: "error while getting google streetview from repository",
			args: args{id: googleMetadata.ID},
			setup: func(f fields, a args) {
				f.repo.On("GetGoogleStreetview", mock.Anything, a.id).
					Return(game.GoogleStreetview{}, errors.New("some repository error"))
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

			got, err := uc.GetGoogleStreetview(t.Context(), tt.args.id)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

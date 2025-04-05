package panorama_test

import (
	"testing"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama"
	"github.com/stretchr/testify/assert"
)

func TestUsecase_CalculateScoreAndDistance(t *testing.T) {
	t.Parallel()

	type args struct {
		provider game.PanoramaProvider
		realLat  float64
		realLng  float64
		userLat  float64
		userLng  float64
	}

	tests := []struct {
		name         string
		args         args
		wantScore    int
		wantDistance int
	}{
		{
			name: "full score",
			args: args{
				provider: "google",
				realLat:  11.22,
				realLng:  22.33,
				userLat:  11.22,
				userLng:  22.33,
			},
			wantScore:    5000,
			wantDistance: 0,
		},
		{
			name: "~60% score (yandex)",
			args: args{
				provider: "yandex",
				realLat:  0.0,
				realLng:  0.0,
				userLat:  4.49758,
				userLng:  0.0,
			},
			wantScore:    3032,
			wantDistance: 500_000,
		},
		{
			name: "~60% score (seznam)",
			args: args{
				provider: "seznam",
				realLat:  0.0,
				realLng:  0.0,
				userLat:  1.34928,
				userLng:  0.0,
			},
			wantScore:    3032,
			wantDistance: 150_000,
		},
		{
			name: "~60% score (google)",
			args: args{
				provider: "google",
				realLat:  0.0,
				realLng:  0.0,
				userLat:  6.74637,
				userLng:  0.0,
			},
			wantScore:    3032,
			wantDistance: 750_000,
		},
		{
			name: "zero score and distance",
			args: args{
				provider: "google",
				realLat:  11.11,
				realLng:  22.22,
				userLat:  0.0,
				userLng:  0.0,
			},
			wantScore:    0,
			wantDistance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := panorama.NewUsecase(panorama.Config{}, nil)

			score, distance := uc.CalculateScoreAndDistance(tt.args.provider,
				tt.args.realLat, tt.args.realLng, tt.args.userLat, tt.args.userLng)
			assert.Equal(t, tt.wantScore, score)
			assert.Equal(t, tt.wantDistance, distance)
		})
	}
}

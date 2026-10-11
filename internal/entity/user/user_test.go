package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestPrivateProfile_ValidateAvatarUpdate(t *testing.T) {
	t.Parallel()

	lastUpdate := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	cooldown := 5 * time.Minute
	tests := []struct {
		name        string
		lastUpdate  time.Time
		requestTime time.Time
		cooldown    time.Duration
		wantErr     error
	}{
		{
			name:        "immediately before cooldown deadline",
			lastUpdate:  lastUpdate,
			requestTime: lastUpdate.Add(cooldown - time.Nanosecond),
			cooldown:    cooldown,
			wantErr:     user.ErrAvatarUpdateTooFrequent,
		},
		{
			name:        "at cooldown deadline",
			lastUpdate:  lastUpdate,
			requestTime: lastUpdate.Add(cooldown),
			cooldown:    cooldown,
		},
		{
			name:        "immediately after cooldown deadline",
			lastUpdate:  lastUpdate,
			requestTime: lastUpdate.Add(cooldown + time.Nanosecond),
			cooldown:    cooldown,
		},
		{
			name:        "avatar never updated",
			requestTime: lastUpdate,
			cooldown:    cooldown,
		},
		{
			name:        "zero last update still has cooldown",
			requestTime: time.Time{},
			cooldown:    cooldown,
			wantErr:     user.ErrAvatarUpdateTooFrequent,
		},
		{
			name:        "zero cooldown at last update",
			lastUpdate:  lastUpdate,
			requestTime: lastUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile := user.PrivateProfile{AvatarLastUpdate: tt.lastUpdate}

			err := profile.ValidateAvatarUpdate(tt.requestTime, tt.cooldown)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)

				return
			}

			assert.NoError(t, err)
		})
	}
}

package user_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestSession_HasUser(t *testing.T) {
	t.Parallel()

	ownerID := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")
	otherID := uuid.MustParse("019a553a-fc00-7000-8000-000000000002")
	tests := []struct {
		name    string
		ownerID uuid.UUID
		userID  uuid.UUID
		wantErr error
	}{
		{name: "matching owner", ownerID: ownerID, userID: ownerID},
		{name: "another user", ownerID: ownerID, userID: otherID, wantErr: user.ErrSessionWrongUser},
		{name: "zero user with nonzero owner", ownerID: ownerID, wantErr: user.ErrSessionWrongUser},
		{name: "nonzero user with zero owner", userID: ownerID, wantErr: user.ErrSessionWrongUser},
		{name: "matching zero ids"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := user.Session{UserID: tt.ownerID}

			err := session.HasUser(tt.userID)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)

				return
			}

			assert.NoError(t, err)
		})
	}
}

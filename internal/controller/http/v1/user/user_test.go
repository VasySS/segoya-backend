package user_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/user"
	"github.com/VasySS/segoya-backend/internal/dto"
	entity "github.com/VasySS/segoya-backend/internal/entity/user"
)

type (
	contextKey struct{}
	tokens     struct{ authorized bool }
)

func (s tokens) FromContext(context.Context) (entity.AccessTokenClaims, bool) {
	return entity.AccessTokenClaims{UserID: profileID()}, s.authorized
}

type usecaseStub struct {
	t       *testing.T
	failure error
	calls   int
}

func (u *usecaseStub) GetPrivateProfile(ctx context.Context, id uuid.UUID) (entity.PrivateProfile, error) {
	u.request(ctx, id)

	return entity.PrivateProfile{
			PublicProfile: entity.PublicProfile{
				ID:         id,
				Username:   "player",
				Name:       "Player",
				AvatarHash: "avatar",
			},
			Password: "secret-hash",
		},
		u.failure
}

func (u *usecaseStub) GetPublicProfile(ctx context.Context, id uuid.UUID) (entity.PublicProfile, error) {
	u.request(ctx, id)
	return entity.PublicProfile{ID: id, Username: "player", Name: "Player", AvatarHash: "avatar"}, u.failure
}

func (u *usecaseStub) UpdateUser(ctx context.Context, req dto.UpdateUserRequest) error {
	u.request(ctx, req.UserID)
	assert.Equal(u.t, "New name", req.Name)

	return u.failure
}

func (u *usecaseStub) UpdateAvatar(ctx context.Context, req dto.UpdateAvatarRequest) error {
	u.request(ctx, req.UserID)
	assert.Equal(u.t, "image/png", req.MimeType)
	assert.WithinDuration(u.t, time.Now().UTC(), req.RequestTime, time.Second)
	data, err := io.ReadAll(req.File)
	require.NoError(u.t, err)
	assert.Equal(u.t, "image bytes", string(data))

	return u.failure
}

func TestHandlerContracts(t *testing.T) {
	t.Parallel()

	type rejection struct {
		name   string
		cause  error
		want   any
		status int
	}

	for _, operation := range []struct {
		name                           string
		invoke                         func(context.Context, *user.Handler) (any, error)
		want, unauthorized, unexpected any
		rejections                     []rejection
	}{
		{
			name: "private profile",
			invoke: func(ctx context.Context,
				h *user.Handler) (any,
				error,
			) {
				return h.GetPrivateProfile(ctx)
			},

			want:         &api.UserPrivateProfile{ID: profileID(), Username: "player", Name: "Player", AvatarHash: "avatar"},
			unauthorized: &api.GetPrivateProfileUnauthorized{}, unexpected: &api.GetPrivateProfileInternalServerError{},
		},
		{
			name: "public profile", invoke: func(ctx context.Context, h *user.Handler) (any, error) {
				return h.GetPublicProfile(ctx, api.GetPublicProfileParams{ID: profileID()})
			},
			want:       &api.UserPublicProfile{ID: profileID(), Username: "player", Name: "Player", AvatarHash: "avatar"},
			unexpected: &api.GetPublicProfileInternalServerError{},
			rejections: []rejection{{"missing user", entity.ErrUserNotFound, &api.GetPublicProfileNotFound{}, 404}},
		},
		{
			name: "update name", invoke: func(ctx context.Context, h *user.Handler) (any, error) {
				return h.UpdateUser(ctx, &api.UserUpdateRequest{Name: api.NewOptString("New name")})
			},
			want:         &api.UpdateUserNoContent{},
			unauthorized: &api.UpdateUserUnauthorized{},
			unexpected:   &api.UpdateUserInternalServerError{},
		},
		{
			name: "update avatar", invoke: func(ctx context.Context, h *user.Handler) (any, error) {
				return h.UpdateUserAvatar(ctx, &api.UpdateUserAvatarReq{AvatarFile: ht.MultipartFile{
					File: strings.NewReader("image bytes"), Header: textproto.MIMEHeader{"Content-Type": {"image/png"}},
				}})
			},
			want:         &api.UpdateUserAvatarNoContent{},
			unauthorized: &api.UpdateUserAvatarUnauthorized{},
			unexpected:   &api.UpdateUserAvatarInternalServerError{},

			rejections: []rejection{{
				"cooldown",
				entity.ErrAvatarUpdateTooFrequent,
				&api.UpdateUserAvatarTooManyRequests{},
				429,
			}},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()

			cases := append([]rejection{
				{
					name: "success",
					want: operation.want,
				},
				{
					"unexpected failure",
					errors.New("private storage detail"),
					operation.unexpected,
					500,
				},
			},
				operation.rejections...)
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					uc := &usecaseStub{t: t}
					if tc.cause != nil {
						uc.failure = fmt.Errorf("wrapped: %w", tc.cause)
					}

					h := user.NewHandler(user.Config{}, uc, tokens{authorized: true})
					ctx := context.WithValue(t.Context(), contextKey{}, "metadata")
					result, err := operation.invoke(ctx, h)
					require.NoError(t, err)
					assert.Equal(t, 1, uc.calls)

					if tc.cause == nil {
						assert.Equal(t, tc.want, result)
						body, err := json.Marshal(result)
						require.NoError(t, err)
						assert.NotContains(t, string(body), "secret-hash")
						assert.NotContains(t, string(body), "password")

						return
					}

					assert.IsType(t, tc.want, result)
					body, err := json.Marshal(result)
					require.NoError(t, err)

					var problem api.BackendError

					require.NoError(t, json.Unmarshal(body, &problem))
					assert.Equal(t, tc.status, problem.Status)
					assert.NotEmpty(t, problem.Title)
					assert.NotEmpty(t, problem.Detail)
					assert.NotContains(t, problem.Detail, "private storage detail")
				})
			}

			if operation.unauthorized != nil {
				t.Run("unauthorized", func(t *testing.T) {
					t.Parallel()
					uc := &usecaseStub{t: t}
					result, err := operation.invoke(t.Context(), user.NewHandler(user.Config{}, uc, tokens{}))
					require.NoError(t, err)
					assert.IsType(t, operation.unauthorized, result)
					assert.Zero(t, uc.calls)
				})
			}
		})
	}
}

func profileID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000001") }

func (u *usecaseStub) request(ctx context.Context, id uuid.UUID) {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(contextKey{}))
	assert.Equal(u.t, profileID(), id)
}

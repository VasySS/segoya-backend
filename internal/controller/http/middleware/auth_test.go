package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/controller/http/middleware"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

type (
	claimsKey   struct{}
	metadataKey struct{}
)

type tokens struct {
	t       *testing.T
	claims  user.AccessTokenClaims
	failure error
	parses  int
}

func (s *tokens) ParseAccessToken(token string) (user.AccessTokenClaims, error) {
	s.parses++
	assert.Equal(s.t, "access-token", token)

	return s.claims, s.failure
}

func (s *tokens) NewContext(ctx context.Context, claims user.AccessTokenClaims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

func (s *tokens) FromContext(ctx context.Context) (user.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(user.AccessTokenClaims)
	return claims, ok
}

func TestBearerAuthentication(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		failure error
	}{
		{name: "valid"}, {name: "invalid", failure: errors.New("invalid signature")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := &tokens{t: t, claims: user.AccessTokenClaims{UserID: uuid.New()}, failure: tc.failure}
			ctx := context.WithValue(t.Context(), metadataKey{}, "metadata")
			result, err := middleware.NewAuth(ts).HandleBearer(ctx, api.OperationName("test"), api.Bearer{Token: "access-token"})
			assert.Equal(t, 1, ts.parses)
			assert.Equal(t, "metadata", result.Value(metadataKey{}))
			claims, ok := ts.FromContext(result)

			if tc.failure != nil {
				require.ErrorIs(t, err, tc.failure)
				assert.False(t, ok)

				return
			}

			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, ts.claims, claims)
		})
	}
}

func TestWebSocketAuthentication(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, query    string
		failure        error
		status, parses int
	}{
		{"missing token", "", nil, 401, 0},
		{"invalid token", "?token=access-token", errors.New("invalid signature"), 401, 1},
		{"valid token", "?token=access-token", nil, 204, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := &tokens{t: t, claims: user.AccessTokenClaims{UserID: uuid.New()}, failure: tc.failure}
			calls := 0
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				claims, ok := ts.FromContext(r.Context())
				assert.True(t, ok)
				assert.Equal(t, ts.claims, claims)
				assert.Equal(t, "metadata", r.Context().Value(metadataKey{}))
				w.WriteHeader(http.StatusNoContent)
			})
			ctx := context.WithValue(t.Context(), metadataKey{}, "metadata")
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/ws"+tc.query, nil)
			recorder := httptest.NewRecorder()
			middleware.NewAuth(ts).HandleWS(next).ServeHTTP(recorder, req)
			assert.Equal(t, tc.status, recorder.Code)
			assert.Equal(t, tc.parses, ts.parses)

			if tc.status == 204 {
				assert.Equal(t, 1, calls)
			} else {
				assert.Zero(t, calls)
			}
		})
	}
}

package http_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	areaHTTP "github.com/VasySS/segoya-backend/internal/controller/http/v1/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

type areaHTTPUsecase struct {
	areaHTTP.Usecase

	t       *testing.T
	failure error
	count   int
	calls   int
}

func (u *areaHTTPUsecase) Count(
	_ context.Context,
	provider game.PanoramaProvider,
	selection playarea.Selection,
	actor uuid.UUID,
) (int, error) {
	u.calls++
	assert.Equal(u.t, routerUserID(), actor)
	assert.Equal(u.t, game.GoogleProvider, provider)
	assert.NotEmpty(u.t, selection.Type)

	return u.count, u.failure
}

func TestPlayAreaCountHTTP(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		body       string
		failure    error
		authorized bool
		status     int
		calls      int
	}{
		{name: "authentication required", body: `{"provider":"google","playArea":{"type":"all"}}`, status: 401},
		{
			name:       "zero matches succeeds",
			body:       `{"provider":"google","playArea":{"type":"all"}}`,
			authorized: true,
			status:     200,
			calls:      1,
		},
		{
			name:       "mixed selectors rejected",
			body:       `{"provider":"google","playArea":{"type":"all","presetIDs":["0199027e-ade6-784d-9d43-cc9c776a6943"]}}`,
			authorized: true,
			status:     400,
		},
		{
			name: "self intersection rejected",
			body: `{
 "provider": "google",
 "playArea": {
  "type": "polygon",
  "polygon": {
   "type": "Polygon",
   "coordinates": [
    [
     [
      0,
      0
     ],
     [
      2,
      2
     ],
     [
      0,
      2
     ],
     [
      2,
      0
     ],
     [
      0,
      0
     ]
    ]
   ]
  }
 }
}`,
			authorized: true,
			status:     400,
		},
		{
			name:       "unknown provider rejected",
			body:       `{"provider":"unknown","playArea":{"type":"all"}}`,
			authorized: true,
			status:     400,
		},
		{
			name:       "private area is hidden",
			body:       `{"provider":"google","playArea":{"type":"saved","savedID":"0199027e-ade6-784d-9d43-cc9c776a6943"}}`,
			failure:    playarea.ErrNotFound,
			authorized: true,
			status:     404,
			calls:      1,
		},
		{
			name:       "unexpected errors are generic",
			body:       `{"provider":"google","playArea":{"type":"all"}}`,
			failure:    errors.New("database secret"),
			authorized: true,
			status:     500,
			calls:      1,
		},
		{name: "request limit", body: strings.Repeat(" ", playarea.MaxBodyBytes+1), authorized: true, status: 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := &areaHTTPUsecase{t: t, failure: tt.failure}

			router, access, _ := newRouter(t, uc)
			if !tt.authorized {
				access = ""
			}

			response := serve(t, router, http.MethodPost, "/v1/play-areas/count", tt.body, access)
			assert.Equal(t, tt.status, response.Code, response.Body.String())
			assert.Equal(t, tt.calls, uc.calls)
			assert.NotContains(t, response.Body.String(), "database secret")

			if tt.status == http.StatusOK {
				assert.JSONEq(t, `{"locationCount":0}`, response.Body.String())
			}
		})
	}
}

func TestPlayAreaEndpointsRequireAuthentication(t *testing.T) {
	t.Parallel()

	id := uuid.New().String()
	for _, tt := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/play-areas?page=1&page-size=10", ""},
		{
			http.MethodPost,
			"/v1/play-areas",
			`{"name":"Home","polygon":{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,1],[0,0]]]}}`,
		},
		{http.MethodGet, "/v1/play-areas/" + id, ""},
		{http.MethodPatch, "/v1/play-areas/" + id, `{"visibility":"unlisted"}`},
		{http.MethodDelete, "/v1/play-areas/" + id, ""},
		{http.MethodGet, "/v1/play-areas/presets?provider=google", ""},
		{http.MethodGet, "/v1/play-areas/presets/" + id, ""},
	} {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			t.Parallel()

			router, _, _ := newRouter(t)
			response := serve(t, router, tt.method, tt.path, tt.body, "")
			require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		})
	}
}

func TestPlayAreaPayloadLimitUsesNormalizedPaths(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"//v1/play-areas/count", "/v1//singleplayer", "/v1/lobbies//"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			router, access, _ := newRouter(t)
			response := serve(t, router, http.MethodPost, path, strings.Repeat(" ", playarea.MaxBodyBytes+1), access)
			require.Equal(t, http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
		})
	}
}

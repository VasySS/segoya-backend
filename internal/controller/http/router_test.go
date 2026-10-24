package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/config"
	controller "github.com/VasySS/segoya-backend/internal/controller/http"
	singleplayerHTTP "github.com/VasySS/segoya-backend/internal/controller/http/v1/singleplayer"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/token"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport/melody"
)

type requestKey struct{}

type gameUsecase struct {
	singleplayerHTTP.Usecase

	t          *testing.T
	calls      int
	newRequest dto.NewSingleplayerGameRequest
	guess      *game.LatLng
	failure    error
}

func (u *gameUsecase) NewGame(ctx context.Context, req dto.NewSingleplayerGameRequest) (uuid.UUID, error) {
	u.checkContext(ctx, req.UserID)
	u.newRequest = req

	return routerGameID(), u.failure
}

func (u *gameUsecase) GetGames(
	ctx context.Context,
	req dto.GetSingleplayerGamesRequest,
) ([]singleplayer.Game, int, error) {
	u.checkContext(ctx, req.UserID)
	assert.Equal(u.t, 2, req.Page)
	assert.Equal(u.t, 3, req.PageSize)

	return nil, 7, u.failure
}

func (u *gameUsecase) EndRound(
	ctx context.Context,
	req dto.EndSingleplayerRoundRequest,
) (dto.EndCurrentRoundResponse, error) {
	u.checkContext(ctx, req.UserID)
	assert.Equal(u.t, routerGameID(), req.GameID)
	assert.WithinDuration(u.t, time.Now().UTC(), req.RequestTime, time.Second)
	u.guess = req.Guess

	return dto.EndCurrentRoundResponse{Score: 321, Distance: 654}, u.failure
}

func newRouter(t *testing.T) (http.Handler, string, *gameUsecase) {
	t.Helper()

	var cfg config.Config

	cfg.ENV.FrontendURL = url.URL{Scheme: "https", Host: "frontend.example"}
	cfg.Limits.AccessTokenTTL, cfg.Limits.RefreshTokenTTL = time.Hour, 24*time.Hour
	ts := token.NewService(t.Context(), "test-signing-key", time.Hour, 24*time.Hour, "", &http.Client{})
	access, err := ts.NewAccessToken(time.Now().UTC(), user.AccessTokenClaims{
		UserID: routerUserID(), SessionID: routerGameID(), Username: "player", Name: "Player",
	})
	require.NoError(t, err)

	lobbyWS, multiplayerWS := melody.NewWebSocketService(), melody.NewWebSocketService()

	t.Cleanup(func() { assert.NoError(t, lobbyWS.Close()) })
	t.Cleanup(func() { assert.NoError(t, multiplayerWS.Close()) })
	uc := &gameUsecase{t: t}

	return controller.NewRouter(cfg, nil, ts, nil, lobbyWS, multiplayerWS, nil, nil, nil, uc, nil), access, uc
}

func serve(t *testing.T, router http.Handler, method, path, body, access string) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.WithValue(t.Context(), requestKey{}, "metadata")

	req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func TestRouterPublicEndpoints(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, method, path string
		status             int
		contentType        string
	}{
		{"root redirect", http.MethodGet, "/", 302, ""},
		{"health", http.MethodGet, "/health", 200, "application/vnd.health+json"},
		{"documentation", http.MethodGet, "/docs", 200, "text/html"},
		{"specification", http.MethodGet, "/openapi/bundled.yaml", 200, "application/yaml"},
		{"CORS preflight", http.MethodOptions, "/v1/singleplayer", 204, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			router, _, uc := newRouter(t)
			response := serve(t, router, tc.method, tc.path, "", "")
			assert.Equal(t, tc.status, response.Code)
			assert.Zero(t, uc.calls)
			assert.Equal(t, "https://frontend.example", response.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "true", response.Header().Get("Access-Control-Allow-Credentials"))

			if tc.contentType != "" {
				assert.Contains(t, response.Header().Get("Content-Type"), tc.contentType)
			}

			switch tc.name {
			case "root redirect":
				assert.Equal(t, "/docs", response.Header().Get("Location"))
			case "health":
				assert.JSONEq(t, `{"status":"OK"}`, response.Body.String())
			case "specification":
				assert.Contains(t, response.Body.String(), "openapi:")
			case "documentation":
				assert.Contains(t, response.Body.String(), "html")
			case "CORS preflight":
				assert.Empty(t, response.Body.String())
				assert.Contains(t, response.Header().Get("Access-Control-Allow-Headers"), "Authorization")
			}
		})
	}
}

func TestRouterAuthorization(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, path, access string }{
		{"missing bearer", "/v1/singleplayer?page=2&page-size=3", ""},
		{"invalid bearer", "/v1/singleplayer?page=2&page-size=3", "invalid"},
		{"missing websocket token", "/v1/lobbies/room/ws", ""},
		{"invalid websocket token", "/v1/multiplayer/" + routerGameID().String() + "/ws?token=invalid", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			router, _, uc := newRouter(t)
			response := serve(t, router, http.MethodGet, tc.path, "", tc.access)
			assert.Equal(t, 401, response.Code)
			assert.Zero(t, uc.calls)
		})
	}
}

func TestRouterRejectsInvalidRequestsBeforeUsecase(t *testing.T) {
	t.Parallel()

	guessPath := "/v1/singleplayer/" + routerGameID().String() + "/round/end"
	for _, tc := range []struct{ name, method, path, body string }{
		{"malformed JSON", http.MethodPost, "/v1/singleplayer", `{"rounds":`},
		{"missing required fields", http.MethodPost, "/v1/singleplayer", `{}`},
		{
			"unknown provider",
			http.MethodPost,
			"/v1/singleplayer",
			`{"provider":"unknown","rounds":1,"movementAllowed":false}`,
		},

		{"invalid rounds", http.MethodPost, "/v1/singleplayer", `{"provider":"google","rounds":0,"movementAllowed":false}`},
		{
			"invalid timer",
			http.MethodPost,
			"/v1/singleplayer",
			`{"provider":"google","rounds":1,"timerSeconds":9,"movementAllowed":false}`,
		},

		{"invalid game ID", http.MethodGet, "/v1/singleplayer/not-a-uuid", ""},
		{"invalid pagination", http.MethodGet, "/v1/singleplayer?page=0&page-size=3", ""},
		{"missing guess", http.MethodPost, guessPath, `{}`},
		{"empty coordinates", http.MethodPost, guessPath, `{"guess":{}}`},
		{"partial coordinates", http.MethodPost, guessPath, `{"guess":{"lat":0}}`},
		{"out of range coordinates", http.MethodPost, guessPath, `{"guess":{"lat":91,"lng":0}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			router, access, uc := newRouter(t)
			response := serve(t, router, tc.method, tc.path, tc.body, access)
			require.Equal(t, 400, response.Code, response.Body.String())
			assert.Contains(t, response.Header().Get("Content-Type"), "application/problem+json")

			var problem struct {
				Title  string `json:"title"`
				Status int    `json:"status"`
				Detail string `json:"detail"`
			}

			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
			assert.Equal(t, 400, problem.Status)
			assert.NotEmpty(t, problem.Title)
			assert.NotEmpty(t, problem.Detail)
			assert.Zero(t, uc.calls)
		})
	}
}

func TestRouterSuccessfulRequests(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, method, path, body, wantJSON string
		status                             int
		timer                              int
		guess                              *game.LatLng
	}{
		{
			name: "untimed game", method: http.MethodPost, path: "/v1/singleplayer",
			body:     `{"provider":"google","rounds":5,"movementAllowed":true}`,
			wantJSON: `{"id":"` + routerGameID().String() + `"}`, status: 201,
		},
		{
			name: "timed game", method: http.MethodPost, path: "/v1/singleplayer",
			body:     `{"provider":"google","rounds":5,"timerSeconds":30,"movementAllowed":true}`,
			wantJSON: `{"id":"` + routerGameID().String() + `"}`, status: 201, timer: 30,
		},
		{
			name: "empty page retains total", method: http.MethodGet,
			path: "/v1/singleplayer?page=2&page-size=3", wantJSON: `{"total":7,"games":[]}`, status: 200,
		},
		{
			name: "null guess", method: http.MethodPost,
			path: "/v1/singleplayer/" + routerGameID().String() + "/round/end",
			body: `{"guess":null}`, wantJSON: `{"score":321,"distance":654}`, status: 200,
		},
		{
			name: "zero coordinates", method: http.MethodPost,
			path: "/v1/singleplayer/" + routerGameID().String() + "/round/end",
			body: `{"guess":{"lat":0,"lng":0}}`, wantJSON: `{"score":321,"distance":654}`, status: 200,
			guess: &game.LatLng{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			router, access, uc := newRouter(t)
			response := serve(t, router, tc.method, tc.path, tc.body, access)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			assert.JSONEq(t, tc.wantJSON, response.Body.String())
			assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
			assert.Equal(t, 1, uc.calls)
			assert.Equal(t, tc.guess, uc.guess)

			if tc.status == 201 {
				assert.Equal(t, "google", uc.newRequest.Provider)
				assert.Equal(t, 5, uc.newRequest.Rounds)
				assert.True(t, uc.newRequest.MovementAllowed)
				assert.Equal(t, tc.timer, uc.newRequest.TimerSeconds)
				assert.WithinDuration(t, time.Now().UTC(), uc.newRequest.RequestTime, time.Second)
			}
		})
	}
}

func TestRouterHidesUnexpectedUsecaseFailure(t *testing.T) {
	t.Parallel()
	router, access, uc := newRouter(t)
	uc.failure = errors.New("private database connection detail")
	response := serve(t, router, http.MethodGet, "/v1/singleplayer?page=2&page-size=3", "", access)
	require.Equal(t, 500, response.Code)
	assert.NotContains(t, response.Body.String(), uc.failure.Error())
	assert.Contains(t, response.Body.String(), `"status":500`)
	assert.Equal(t, 1, uc.calls)
}

func routerUserID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000001") }

func routerGameID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000002") }

func (u *gameUsecase) checkContext(ctx context.Context, id uuid.UUID) {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, routerUserID(), id)
	assert.Equal(u.t, "metadata", ctx.Value(requestKey{}))
}

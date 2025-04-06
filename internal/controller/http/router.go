// Package http provides a router for handling HTTP requests.
package http

import (
	"context"
	"log"
	"net/http"

	apiembed "github.com/VasySS/segoya-backend/api"
	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/config"
	"github.com/VasySS/segoya-backend/internal/controller/http/middleware"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/auth"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/lobby"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/multiplayer"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/singleplayer"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/token"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	"github.com/VasySS/segoya-backend/pkg/captcha"
	"github.com/VasySS/segoya-backend/pkg/crypto"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// APIHandler is a wrapper for all API handlers to implement ogen's api.Handler interface.
type APIHandler struct {
	api.UsersHandler
	api.AuthHandler
	api.LobbiesHandler
	api.SingleplayerHandler
	api.MultiplayerHandler
}

func newAPIHandler(
	uh api.UsersHandler,
	ah api.AuthHandler,
	lh api.LobbiesHandler,
	sh api.SingleplayerHandler,
	mh api.MultiplayerHandler,
) *APIHandler {
	return &APIHandler{
		UsersHandler:        uh,
		AuthHandler:         ah,
		LobbiesHandler:      lh,
		SingleplayerHandler: sh,
		MultiplayerHandler:  mh,
	}
}

// GetRoot redirects to the documentation page from the root endpoint.
func (h *APIHandler) GetRoot(_ context.Context) (*api.GetRootFound, error) {
	return &api.GetRootFound{Location: "/docs"}, nil
}

// GetHealth returns the health status of the API.
func (h *APIHandler) GetHealth(_ context.Context) (*api.GetHealthOK, error) {
	return &api.GetHealthOK{Status: "OK"}, nil
}

// NewRouter initializes a new http router and registers all handlers.
func NewRouter(
	conf config.Config,
	randomService *crypto.Service,
	tokenService *token.Service,
	captchaService *captcha.CloudflareService,
	lobbyWSService transport.WebSocketService,
	multiplayerWSService transport.WebSocketService,
	authUsecase auth.Usecase,
	userUsecase user.Usecase,
	lobbyUsecase lobby.Usecase,
	singleplayerUsecase singleplayer.Usecase,
	multiplayerUsecase multiplayer.Usecase,
) http.Handler {
	mux := chi.NewMux()

	mux.Use(
		chiMiddleware.RequestID,
		middleware.Logger,
		chiMiddleware.Recoverer,
		middleware.CORS(conf.ENV.FrontendURL.String()),
		chiMiddleware.CleanPath,
		chiMiddleware.StripSlashes,
		middleware.Compress,
	)

	uh := user.NewHandler(user.NewConfig(conf), userUsecase, tokenService)
	ah := auth.NewHandler(auth.NewConfig(conf), authUsecase, randomService, tokenService, captchaService)
	lh := lobby.NewHandler(lobby.NewConfig(conf), lobbyUsecase, tokenService, lobbyWSService)
	sh := singleplayer.NewHandler(singleplayer.NewConfig(conf), singleplayerUsecase, tokenService)
	mh := multiplayer.NewHandler(multiplayer.NewConfig(conf), multiplayerUsecase, tokenService, multiplayerWSService)

	authMW := middleware.NewAuth(tokenService)

	ogenServer, err := api.NewServer(
		newAPIHandler(uh, ah, lh, sh, mh),
		authMW,
		api.WithErrorHandler(middleware.ErrorHandler),
		api.WithMiddleware(middleware.OpenTelemetry{}.Middleware),
	)
	if err != nil {
		log.Fatalf("failed to create ogen server: %v", err)
	}

	mux.Mount("/", ogenServer)
	mux.With(authMW.HandleWS).HandleFunc("/v1/lobbies/{id}/ws", lh.HandleWS)
	mux.With(authMW.HandleWS).HandleFunc("/v1/multiplayer/{id}/ws", mh.HandleWS)

	mux.HandleFunc("/openapi/bundled.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(apiembed.OpenAPISpec)
	})

	mux.HandleFunc("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(apiembed.OpenAPIDocsHTML)
	})

	return mux
}

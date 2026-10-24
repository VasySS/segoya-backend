package multiplayer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	handler "github.com/VasySS/segoya-backend/internal/controller/http/v1/multiplayer"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

type handlerUsecase struct {
	handler.Usecase

	getGame    func(context.Context, uuid.UUID, uuid.UUID) (multiplayer.Game, error)
	getGuesses func(context.Context, dto.GetGameGuessesRequest) ([]multiplayer.Guess, error)
	newRound   func(context.Context, dto.NewMultiplayerRoundRequest) (multiplayer.Round, error)
	getRound   func(context.Context, dto.GetMultiplayerRoundRequest) (multiplayer.Round, error)
	getUser    func(context.Context, uuid.UUID, uuid.UUID) (user.MultiplayerUser, error)
	getUsers   func(context.Context, uuid.UUID) ([]user.MultiplayerUser, error)
	guess      func(context.Context, dto.NewMultiplayerRoundGuessRequest) error
	end        func(context.Context, dto.EndMultiplayerRoundRequest) ([]multiplayer.Guess, error)
}

func (u handlerUsecase) GetGame(ctx context.Context, gameID, userID uuid.UUID) (multiplayer.Game, error) {
	return u.getGame(ctx, gameID, userID)
}

func (u handlerUsecase) GetGameGuesses(
	ctx context.Context, req dto.GetGameGuessesRequest,
) ([]multiplayer.Guess, error) {
	return u.getGuesses(ctx, req)
}

func (u handlerUsecase) NewRound(ctx context.Context, req dto.NewMultiplayerRoundRequest) (multiplayer.Round, error) {
	return u.newRound(ctx, req)
}

func (u handlerUsecase) GetRound(ctx context.Context, req dto.GetMultiplayerRoundRequest) (multiplayer.Round, error) {
	return u.getRound(ctx, req)
}

func (u handlerUsecase) GetGameUser(ctx context.Context, uid, gameID uuid.UUID) (user.MultiplayerUser, error) {
	return u.getUser(ctx, uid, gameID)
}

func (u handlerUsecase) GetGameUsers(ctx context.Context, gameID uuid.UUID) ([]user.MultiplayerUser, error) {
	return u.getUsers(ctx, gameID)
}

func (u handlerUsecase) NewRoundGuess(ctx context.Context, req dto.NewMultiplayerRoundGuessRequest) error {
	return u.guess(ctx, req)
}

func (u handlerUsecase) EndRound(ctx context.Context, req dto.EndMultiplayerRoundRequest) ([]multiplayer.Guess, error) {
	return u.end(ctx, req)
}

func multiplayerSession(t *testing.T, room string, profile any) *wsfake.Session {
	t.Helper()
	session := &wsfake.Session{Req: httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)}
	session.SetBroadcastID(room)

	if profile != nil {
		session.Set(dto.MultiplayerUserProfileKey, profile)
	}

	return session
}

func connectingMultiplayerSession(t *testing.T, gameID string) *wsfake.Session {
	t.Helper()

	route := chi.NewRouteContext()
	route.URLParams.Add("id", gameID)
	ctx := context.WithValue(t.Context(), chi.RouteCtxKey, route)

	return &wsfake.Session{Req: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)}
}

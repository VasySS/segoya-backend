package multiplayer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/controller/http/v1/multiplayer"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

type admissionUsecase struct {
	multiplayer.Usecase

	profile user.MultiplayerUser
	denied  bool
	lookups int
}

func (u *admissionUsecase) GetGameUser(context.Context, uuid.UUID, uuid.UUID) (user.MultiplayerUser, error) {
	if u.denied {
		return user.MultiplayerUser{}, errors.New("not a participant")
	}

	return u.profile, nil
}

func (u *admissionUsecase) GetGameUsers(context.Context, uuid.UUID) ([]user.MultiplayerUser, error) {
	u.lookups++
	return []user.MultiplayerUser{u.profile}, nil
}

func TestMultiplayerAdmission(t *testing.T) {
	t.Parallel()

	for _, denied := range []bool{true, false} {
		t.Run(map[bool]string{true: "outsider", false: "participant"}[denied], func(t *testing.T) {
			t.Parallel()

			gameID := uuid.New()
			route := chi.NewRouteContext()
			route.URLParams.Add("id", gameID.String())
			ctx := context.WithValue(t.Context(), chi.RouteCtxKey, route)
			session := &wsfake.Session{Req: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)}
			service := &wsfake.Service{}
			service.SetSessions(session)

			uc := &admissionUsecase{
				denied:  denied,
				profile: user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New()}},
			}
			session.BeforeSend = func() { require.Empty(t, session.Snapshot().BroadcastID) }

			tokens := wsfake.Tokens{Authorized: true, Claims: user.AccessTokenClaims{UserID: uc.profile.ID}}
			multiplayer.NewHandler(multiplayer.Config{}, uc, tokens, service)
			service.Connect(session)

			if denied {
				require.True(t, session.Snapshot().Closed)
				require.Empty(t, session.Snapshot().BroadcastID)
				require.Zero(t, uc.lookups)
				service.Message(session, transport.WebSocketMessageInput{
					Type:    dto.MultiplayerMessageUserGuess,
					Payload: json.RawMessage(`{"guess":null}`),
				})
				require.Empty(t, service.Broadcasts())
			} else {
				require.False(t, session.Snapshot().Closed)
				require.Equal(t, gameID.String(), session.Snapshot().BroadcastID)
				require.Len(t, service.Broadcasts(), 1)
			}
		})
	}
}

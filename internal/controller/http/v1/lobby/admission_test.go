package lobby_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/controller/http/v1/lobby"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

type admissionUsecase struct {
	lobby.Usecase

	connect func(context.Context, string, uuid.UUID, string) (user.PublicProfile, error)
	release func(context.Context, string, uuid.UUID, string) error
}

func (u admissionUsecase) ConnectLobbyUser(
	ctx context.Context,
	id string,
	uid uuid.UUID,
	sid string,
) (user.PublicProfile, error) {
	return u.connect(ctx, id, uid, sid)
}

func (u admissionUsecase) DisconnectLobbyUser(ctx context.Context, id string, uid uuid.UUID, sid string) error {
	return u.release(ctx, id, uid, sid)
}

func TestAdmissionInitialization(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"unauthorized", "rejected", "synchronization failure", "success"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			route := chi.NewRouteContext()
			route.URLParams.Add("id", "lobby")
			ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
			session := &wsfake.Session{Req: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)}
			service := &wsfake.Service{}
			service.SetSessions(session)

			profile := user.PublicProfile{ID: uuid.New(), Username: "authenticated"}
			released := 0
			uc := admissionUsecase{
				connect: func(_ context.Context, id string, uid uuid.UUID, sid string) (user.PublicProfile, error) {
					require.Equal(t, "lobby", id)
					require.Equal(t, profile.ID, uid)
					require.Equal(t, session.ID(), sid)

					if scenario == "rejected" {
						return user.PublicProfile{}, errors.New("full lobby")
					}

					return profile, nil
				},
				release: func(ctx context.Context, _ string, uid uuid.UUID, sid string) error {
					require.NoError(t, ctx.Err())
					require.Equal(t, profile.ID, uid)
					require.Equal(t, session.ID(), sid)

					released++

					return nil
				},
			}

			session.BeforeSend = func() { require.Empty(t, session.Snapshot().BroadcastID) }
			if scenario == "synchronization failure" {
				session.SendErr = errors.New("socket failed")
			}

			lobby.NewHandler(lobby.Config{}, uc, wsfake.Tokens{
				Authorized: scenario != "unauthorized",
				Claims:     user.AccessTokenClaims{UserID: profile.ID},
			}, service)
			service.Connect(session)

			if scenario == "success" {
				require.False(t, session.Snapshot().Closed)
				require.Equal(t, "lobby", session.Snapshot().BroadcastID)

				before := time.Now().UTC()

				service.Message(session, transport.WebSocketMessageInput{
					Type:    dto.LobbyMessageTypeChatInput,
					Payload: json.RawMessage(`{"message":{"username":"spoofed","text":"hello","time":"2000-01-01T00:00:00Z"}}`),
				})

				broadcasts := service.Broadcasts()
				require.Len(t, broadcasts, 2)
				message, ok := broadcasts[1].Message.Payload["message"].(dto.LobbyChatMessage)
				require.True(t, ok)
				require.Equal(t, profile.Username, message.Username)
				require.False(t, message.Time.Before(before))
			} else {
				require.True(t, session.Snapshot().Closed)
				require.Empty(t, session.Snapshot().BroadcastID)
				service.Message(session, transport.WebSocketMessageInput{
					Type:    dto.LobbyMessageTypeChatInput,
					Payload: json.RawMessage(`{"message":{}}`),
				})
				require.Empty(t, service.Broadcasts())
			}

			cancel()
			service.Disconnect(session)

			switch scenario {
			case "success":
				require.Equal(t, 1, released)
			case "synchronization failure", "rejected":
				require.Equal(t, 2, released)
			default:
				require.Zero(t, released)
			}
		})
	}
}

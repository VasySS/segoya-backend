package lobby_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/controller/http/v1/lobby"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

func connectingLobbySession(ctx context.Context) *wsfake.Session {
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "lobby")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, route)

	return &wsfake.Session{Req: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)}
}

func TestLobbyConnectSynchronizesOnlyAdmittedUsersInItsRoom(t *testing.T) {
	t.Parallel()

	profile := user.PublicProfile{ID: uuid.New(), Username: "joining"}
	other := user.PublicProfile{ID: uuid.New(), Username: "existing"}
	session := connectingLobbySession(t.Context())
	service := &wsfake.Service{}
	service.SetSessions(session, lobbySession(t, "lobby", other),
		lobbySession(t, "different-lobby", profile), lobbySession(t, "", profile),
		lobbySession(t, "lobby", nil), lobbySession(t, "lobby", "wrong type"),
		lobbySession(t, "lobby", user.PublicProfile{}))

	uc := admissionUsecase{connect: func(
		ctx context.Context, room string, uid uuid.UUID, sid string,
	) (user.PublicProfile, error) {
		assert.Equal(t, "lobby", room)
		assert.Equal(t, profile.ID, uid)
		assert.Equal(t, session.ID(), sid)
		require.NoError(t, ctx.Err())

		return profile, nil
	}}
	session.BeforeSend = func() {
		assert.Empty(t, session.Snapshot().BroadcastID)
		_, admitted := session.Get(dto.LobbyUserProfileKey)
		assert.False(t, admitted, "initial synchronization must finish before the session is admitted")
	}

	lobby.NewHandler(lobby.Config{}, uc, wsfake.Tokens{
		Authorized: true, Claims: user.AccessTokenClaims{UserID: profile.ID},
	}, service)
	service.Connect(session)

	state := session.Snapshot()
	assert.False(t, state.Closed)
	assert.Empty(t, state.Errors)
	assert.Equal(t, profile, state.Values[dto.LobbyUserProfileKey])
	assert.Equal(t, "lobby", state.BroadcastID)
	assert.Equal(t, []transport.WebSocketMessageOutput{{
		Type: dto.LobbyMessageTypeConnectedUsers, Payload: map[string]any{"users": []user.PublicProfile{other}},
	}}, state.Messages)
	assert.Equal(t, []wsfake.Broadcast{{RoomID: "lobby", Message: transport.WebSocketMessageOutput{
		Type: dto.LobbyMessageTypeUserConnected, Payload: map[string]any{"user": profile},
	}}}, service.Broadcasts())
}

func TestLobbyAdmissionFailureCleanup(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name           string
		unauthorized   bool
		canceled       bool
		connectErr     error
		sendErr        error
		releaseErr     error
		wantError      string
		wantConnects   int
		wantReleases   int
		wantDirectSend int
	}{
		{name: "unauthorized", unauthorized: true, wantError: "error authorizing user"},
		{
			name: "admission rejected", connectErr: errors.New("lobby full"),
			wantError: "error connecting to lobby", wantConnects: 1, wantReleases: 2,
		},
		{
			name: "canceled admission still releases reservation", canceled: true,
			wantError: "error connecting to lobby", wantConnects: 1, wantReleases: 2,
		},
		{
			name: "initial synchronization fails", sendErr: errors.New("connection closed"),
			wantError: "error connecting to lobby", wantConnects: 1, wantReleases: 2, wantDirectSend: 1,
		},
		{
			name: "release failure keeps rejection generic", connectErr: errors.New("reply lost"),
			releaseErr: errors.New("storage unavailable"),
			wantError:  "error connecting to lobby", wantConnects: 1, wantReleases: 2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if tt.canceled {
				cancel()
			}

			session := connectingLobbySession(ctx)
			session.SendErr = tt.sendErr
			service := &wsfake.Service{}
			profile := user.PublicProfile{ID: uuid.New(), Username: "player"}
			connects, releases := 0, 0
			uc := admissionUsecase{
				connect: func(ctx context.Context, _ string, _ uuid.UUID, _ string) (user.PublicProfile, error) {
					connects++

					if err := ctx.Err(); err != nil {
						return user.PublicProfile{}, fmt.Errorf("admission canceled: %w", err)
					}

					return profile, tt.connectErr
				},
				release: func(ctx context.Context, room string, uid uuid.UUID, sid string) error {
					releases++

					require.NoError(t, ctx.Err())
					deadline, ok := ctx.Deadline()
					assert.True(t, ok, "cleanup must have its own deadline")
					assert.LessOrEqual(t, time.Until(deadline), 5*time.Second)
					assert.Equal(t, "lobby", room)
					assert.Equal(t, profile.ID, uid)
					assert.Equal(t, session.ID(), sid)

					return tt.releaseErr
				},
			}
			lobby.NewHandler(lobby.Config{}, uc, wsfake.Tokens{
				Authorized: !tt.unauthorized, Claims: user.AccessTokenClaims{UserID: profile.ID},
			}, service)
			service.Connect(session)
			cancel()
			service.Disconnect(session)

			state := session.Snapshot()
			assert.True(t, state.Closed)
			assert.Empty(t, state.BroadcastID)
			assert.NotContains(t, state.Values, dto.LobbyUserProfileKey)
			assert.Equal(t, []string{tt.wantError}, state.Errors)
			assert.Len(t, state.Messages, tt.wantDirectSend)
			assert.Equal(t, tt.wantConnects, connects)
			assert.Equal(t, tt.wantReleases, releases)
			assert.Empty(t, service.Broadcasts())
		})
	}
}

func TestLobbyDisconnectBroadcastsEvenWhenReleaseFails(t *testing.T) {
	t.Parallel()

	for _, failed := range []bool{false, true} {
		name := "release succeeds"
		if failed {
			name = "release fails"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			session := connectingLobbySession(ctx)
			service := &wsfake.Service{}
			profile := user.PublicProfile{ID: uuid.New(), Username: "leaving"}
			calls := 0
			uc := admissionUsecase{
				connect: func(context.Context, string, uuid.UUID, string) (user.PublicProfile, error) { return profile, nil },
				release: func(ctx context.Context, _ string, _ uuid.UUID, _ string) error {
					calls++

					require.NoError(t, ctx.Err())

					if failed {
						return errors.New("release failed")
					}

					return nil
				},
			}
			lobby.NewHandler(lobby.Config{}, uc, wsfake.Tokens{Authorized: true}, service)
			service.Connect(session)
			cancel()
			service.Disconnect(session)

			assert.Equal(t, 1, calls)

			broadcasts := service.Broadcasts()
			require.Len(t, broadcasts, 2)
			assert.Equal(t, wsfake.Broadcast{
				RoomID: "lobby", ExcludedSessionID: session.ID(),
				Message: transport.WebSocketMessageOutput{
					Type: dto.LobbyMessageTypeUserDisconnected, Payload: map[string]any{"username": profile.Username},
				},
			}, broadcasts[1])
		})
	}
}

func TestLobbyMessagesRejectUnadmittedSessionsAndMalformedChat(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, room, payload, wantError string
		profile                        any
		messageType                    transport.WebSocketMessageInputType
	}{
		{name: "missing room", profile: user.PublicProfile{Username: "user"}, wantError: "lobby id not found"},
		{name: "missing profile", room: "lobby", wantError: "error getting user profile"},
		{name: "wrong profile type", room: "lobby", profile: "wrong", wantError: "error getting user profile"},
		{name: "empty profile", room: "lobby", profile: user.PublicProfile{}, wantError: "error getting user profile"},
		{
			name: "malformed chat", room: "lobby", profile: user.PublicProfile{Username: "user"},
			messageType: dto.LobbyMessageTypeChatInput, payload: `{"message":`,
			wantError: "error unmarshalling chat message",
		},
		{
			name: "unknown message ignored", room: "lobby",
			profile: user.PublicProfile{Username: "user"}, messageType: "unknown",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := lobbySession(t, tt.room, tt.profile)
			service := &wsfake.Service{}
			lobby.NewHandler(lobby.Config{}, nil, nil, service)
			service.Message(session, transport.WebSocketMessageInput{Type: tt.messageType, Payload: []byte(tt.payload)})

			if tt.wantError == "" {
				assert.Empty(t, session.Snapshot().Errors)
			} else {
				assert.Equal(t, []string{tt.wantError}, session.Snapshot().Errors)
			}

			assert.Empty(t, service.Broadcasts())
			assert.Empty(t, session.Snapshot().Messages)
		})
	}
}

type gameStartUsecase struct {
	lobby.Usecase

	start func(context.Context, dto.StartLobbyGameRequest) (uuid.UUID, error)
}

func (u gameStartUsecase) StartLobbyGame(ctx context.Context, req dto.StartLobbyGameRequest) (uuid.UUID, error) {
	return u.start(ctx, req)
}

func TestLobbyGameStart(t *testing.T) {
	t.Parallel()

	for _, failed := range []bool{false, true} {
		name := "successful redirect"
		if failed {
			name = "usecase failure"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			creator := user.PublicProfile{ID: uuid.New(), Username: "creator"}
			other := user.PublicProfile{ID: uuid.New(), Username: "other"}
			session := lobbySession(t, "lobby", creator)
			service := &wsfake.Service{}
			service.SetSessions(session, lobbySession(t, "lobby", other), lobbySession(t, "elsewhere", other))

			gameID, calls := uuid.New(), 0
			before := time.Now()
			uc := gameStartUsecase{start: func(ctx context.Context, req dto.StartLobbyGameRequest) (uuid.UUID, error) {
				calls++

				require.NoError(t, ctx.Err())
				assert.Equal(t, "lobby", req.LobbyID)
				assert.Equal(t, creator, req.Creator)
				assert.Equal(t, []user.PublicProfile{creator, other}, req.ConnectedPlayers)
				assert.False(t, req.RequestTime.Before(before))

				if failed {
					return uuid.Nil, errors.New("storage details")
				}

				return gameID, nil
			}}
			lobby.NewHandler(lobby.Config{}, uc, nil, service)
			service.Message(session, transport.WebSocketMessageInput{Type: dto.LobbyMessageTypeGameStart})
			assert.Equal(t, 1, calls)

			if failed {
				assert.Equal(t, []string{"error starting game"}, session.Snapshot().Errors)
				assert.Empty(t, service.Broadcasts())

				return
			}

			assert.Empty(t, session.Snapshot().Errors)
			assert.Equal(t, []wsfake.Broadcast{{RoomID: "lobby", Message: transport.WebSocketMessageOutput{
				Type: dto.LobbyMessageTypeGameRedirect, Payload: map[string]any{"gameID": gameID.String()},
			}}}, service.Broadcasts())
		})
	}
}

func TestLobbyWebSocketUpgradeDelegatesToTransport(t *testing.T) {
	t.Parallel()

	for _, transportErr := range []error{nil, errors.New("upgrade failed")} {
		name := "success"
		if transportErr != nil {
			name = "transport failure"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &wsfake.Service{RequestErr: transportErr}
			handler := lobby.NewHandler(lobby.Config{}, nil, nil, service)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			handler.HandleWS(httptest.NewRecorder(), req)
			require.Len(t, service.Requests(), 1)
			assert.Same(t, req, service.Requests()[0])
		})
	}
}

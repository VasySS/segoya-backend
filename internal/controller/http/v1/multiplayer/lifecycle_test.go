package multiplayer_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	handler "github.com/VasySS/segoya-backend/internal/controller/http/v1/multiplayer"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

func TestMultiplayerAdmissionFailures(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, wantError                          string
		invalidID, unauthorized, duplicate       bool
		userErr, usersErr, sendErr               error
		wantUserCalls, wantUsersCalls, wantSends int
	}{
		{name: "invalid game id", invalidID: true, wantError: "incorrect game id"},
		{name: "unauthorized", unauthorized: true, wantError: "error authorizing user"},
		{
			name: "profile unavailable", userErr: errors.New("database details"),
			wantError: "error connecting to game", wantUserCalls: 1,
		},
		{
			name: "users unavailable", usersErr: errors.New("database details"),
			wantError: "error getting game users", wantUserCalls: 1, wantUsersCalls: 1,
		},
		{
			name: "duplicate connection", duplicate: true,
			wantError: "a connection to this game already exists for user", wantUserCalls: 1, wantUsersCalls: 1,
		},
		{
			name: "initial synchronization fails", sendErr: errors.New("socket closed"),
			wantError: "error getting game users", wantUserCalls: 1, wantUsersCalls: 1, wantSends: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gameID := uuid.New()

			id := gameID.String()
			if tt.invalidID {
				id = "not-a-uuid"
			}

			session := connectingMultiplayerSession(t, id)
			session.SendErr = tt.sendErr
			profile := user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New(), Username: "player"}}

			service := &wsfake.Service{}

			if tt.duplicate {
				service.SetSessions(multiplayerSession(t, id, profile))
			}

			userCalls, usersCalls := 0, 0
			uc := handlerUsecase{
				getUser: func(_ context.Context, uid, gid uuid.UUID) (user.MultiplayerUser, error) {
					userCalls++

					assert.Equal(t, profile.ID, uid)
					assert.Equal(t, gameID, gid)

					return profile, tt.userErr
				},
				getUsers: func(_ context.Context, gid uuid.UUID) ([]user.MultiplayerUser, error) {
					usersCalls++

					assert.Equal(t, gameID, gid)

					return []user.MultiplayerUser{profile}, tt.usersErr
				},
			}
			handler.NewHandler(handler.Config{}, uc, wsfake.Tokens{
				Authorized: !tt.unauthorized, Claims: user.AccessTokenClaims{UserID: profile.ID},
			}, service)
			service.Connect(session)
			state := session.Snapshot()
			assert.True(t, state.Closed)
			assert.Empty(t, state.BroadcastID)
			assert.NotContains(t, state.Values, dto.MultiplayerUserProfileKey)
			assert.Equal(t, []string{tt.wantError}, state.Errors)
			assert.Len(t, state.Messages, tt.wantSends)
			assert.Equal(t, tt.wantUserCalls, userCalls)
			assert.Equal(t, tt.wantUsersCalls, usersCalls)
			assert.Empty(t, service.Broadcasts())
			service.Disconnect(session)
			assert.Empty(t, service.Broadcasts(), "rejected sessions cannot announce disconnection")
		})
	}
}

func TestMultiplayerInitialUserSnapshotFiltersRoomsAndProfiles(t *testing.T) {
	t.Parallel()

	gameID := uuid.New().String()
	joining := user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New(), Username: "joining"}}
	online := user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New(), Username: "online"}}
	offline := user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New(), Username: "offline"}}
	users := []user.MultiplayerUser{joining, online, offline}
	session := connectingMultiplayerSession(t, gameID)
	service := &wsfake.Service{}
	service.SetSessions(session, multiplayerSession(t, gameID, online), multiplayerSession(t, "another-game", offline),
		multiplayerSession(t, "", offline), multiplayerSession(t, gameID, nil),
		multiplayerSession(t, gameID, user.MultiplayerUser{}), multiplayerSession(t, gameID, "wrong profile type"))

	uc := handlerUsecase{
		getUser:  func(context.Context, uuid.UUID, uuid.UUID) (user.MultiplayerUser, error) { return joining, nil },
		getUsers: func(context.Context, uuid.UUID) ([]user.MultiplayerUser, error) { return slices.Clone(users), nil },
	}
	session.BeforeSend = func() {
		assert.Empty(t, session.Snapshot().BroadcastID)
		_, admitted := session.Get(dto.MultiplayerUserProfileKey)
		assert.False(t, admitted)
	}

	handler.NewHandler(handler.Config{}, uc, wsfake.Tokens{
		Authorized: true, Claims: user.AccessTokenClaims{UserID: joining.ID},
	}, service)
	service.Connect(session)
	state := session.Snapshot()
	wantUsers := slices.Clone(users)
	wantUsers[1].Connected = true

	assert.False(t, state.Closed)
	assert.Empty(t, state.Errors)
	assert.Equal(t, gameID, state.BroadcastID)
	assert.Equal(t, joining, state.Values[dto.MultiplayerUserProfileKey])
	assert.Equal(t, []transport.WebSocketMessageOutput{{
		Type: dto.MultiplayerMessageConnectedUsers, Payload: map[string]any{"users": wantUsers},
	}}, state.Messages)
	assert.Equal(t, []wsfake.Broadcast{{RoomID: gameID, Message: transport.WebSocketMessageOutput{
		Type: dto.MultiplayerMessageUserConnected, Payload: map[string]any{"user": joining},
	}}}, service.Broadcasts())
}

func TestMultiplayerGuessMessages(t *testing.T) {
	t.Parallel()

	invalidPayload := "invalid guess: use guess:null or an object with valid lat and lng"
	for _, tt := range []struct {
		name, payload, wantError string
		guess                    *game.LatLng
		usecaseErr               error
		wantCalls                int
	}{
		{
			name: "coordinates", payload: `{"guess":{"lat":55.5,"lng":37.5}}`,
			guess: &game.LatLng{Lat: 55.5, Lng: 37.5}, wantCalls: 1,
		},
		{name: "zero coordinates are a guess", payload: `{"guess":{"lat":0,"lng":0}}`, guess: &game.LatLng{}, wantCalls: 1},
		{name: "null guess", payload: `{"guess":null}`, wantCalls: 1},
		{name: "malformed JSON", payload: `{"guess":`, wantError: invalidPayload},
		{name: "omitted guess", payload: `{}`, wantError: invalidPayload},
		{name: "incomplete coordinates", payload: `{"guess":{"lat":1}}`, wantError: invalidPayload},
		{name: "coordinate type mismatch", payload: `{"guess":{"lat":"1","lng":2}}`, wantError: invalidPayload},
		{name: "out of range", payload: `{"guess":{"lat":91,"lng":2}}`, wantError: invalidPayload},
		{
			name: "round not started", payload: `{"guess":null}`,
			usecaseErr: fmt.Errorf("wrapped: %w", game.ErrRoundNotStarted),
			wantError:  "round has not started", wantCalls: 1,
		},
		{
			name: "invalid coordinates from usecase", payload: `{"guess":null}`,
			usecaseErr: fmt.Errorf("wrapped: %w", game.ErrInvalidCoordinates),
			wantError:  "invalid coordinates", wantCalls: 1,
		},
		{
			name: "round finished", payload: `{"guess":null}`,
			usecaseErr: fmt.Errorf("wrapped: %w", multiplayer.ErrRoundAlreadyFinished),
			wantError:  "round already finished", wantCalls: 1,
		},
		{
			name: "unexpected failure", payload: `{"guess":null}`, usecaseErr: errors.New("database details"),
			wantError: "error saving guess", wantCalls: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gameID := uuid.New()
			profile := user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New(), Username: "authenticated"}}
			session := multiplayerSession(t, gameID.String(), profile)
			service := &wsfake.Service{}

			var requests []dto.NewMultiplayerRoundGuessRequest

			uc := handlerUsecase{guess: func(_ context.Context, req dto.NewMultiplayerRoundGuessRequest) error {
				requests = append(requests, req)
				return tt.usecaseErr
			}}
			handler.NewHandler(handler.Config{}, uc, nil, service)

			before := time.Now()

			service.Message(session, transport.WebSocketMessageInput{
				Type: dto.MultiplayerMessageUserGuess, Payload: []byte(tt.payload),
			})
			require.Len(t, requests, tt.wantCalls)

			if tt.wantCalls != 0 {
				assert.Equal(t, gameID, requests[0].GameID)
				assert.Equal(t, profile.ID, requests[0].UserID)
				assert.Equal(t, tt.guess, requests[0].Guess)
				assert.False(t, requests[0].RequestTime.Before(before))
				assert.False(t, requests[0].RequestTime.After(time.Now()))
			}

			if tt.wantError != "" {
				assert.Equal(t, []string{tt.wantError}, session.Snapshot().Errors)
				assert.Empty(t, service.Broadcasts())

				return
			}

			assert.Empty(t, session.Snapshot().Errors)
			assert.Empty(t, session.Snapshot().Messages)
			assert.Equal(t, []wsfake.Broadcast{{RoomID: gameID.String(), Message: transport.WebSocketMessageOutput{
				Type: dto.MultiplayerMessageUserGuessed, Payload: map[string]any{"username": profile.Username},
			}}}, service.Broadcasts())
		})
	}
}

func TestMultiplayerMessageAdmissionGuards(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, room, wantError string
		profile               any
		messageType           transport.WebSocketMessageInputType
	}{
		{
			name: "missing room", profile: user.MultiplayerUser{PublicProfile: user.PublicProfile{Username: "player"}},
			wantError: "game id not found",
		},
		{name: "missing profile", room: "game", wantError: "user is not admitted to game"},
		{name: "empty profile", room: "game", profile: user.MultiplayerUser{}, wantError: "user is not admitted to game"},
		{name: "wrong profile type", room: "game", profile: "wrong", wantError: "user is not admitted to game"},
		{
			name: "invalid game id for guess", room: "invalid",
			profile:     user.MultiplayerUser{PublicProfile: user.PublicProfile{Username: "player"}},
			messageType: dto.MultiplayerMessageUserGuess, wantError: "incorrect game id",
		},
		{
			name: "invalid game id for round end", room: "invalid",
			profile:     user.MultiplayerUser{PublicProfile: user.PublicProfile{Username: "player"}},
			messageType: dto.MultiplayerMessageRoundEnd, wantError: "incorrect game id",
		},
		{
			name: "unknown message", room: "game",
			profile:     user.MultiplayerUser{PublicProfile: user.PublicProfile{Username: "player"}},
			messageType: "unknown", wantError: "unknown message type",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := multiplayerSession(t, tt.room, tt.profile)
			service := &wsfake.Service{}
			handler.NewHandler(handler.Config{}, nil, nil, service)
			service.Message(session, transport.WebSocketMessageInput{Type: tt.messageType, Payload: []byte(`{"guess":null}`)})
			assert.Equal(t, []string{tt.wantError}, session.Snapshot().Errors)
			assert.Empty(t, service.Broadcasts())
		})
	}
}

func TestMultiplayerRoundEndRepliesOnlyToRequester(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                string
		usecaseErr, sendErr error
		wantSends           int
	}{
		{name: "completed round", wantSends: 1},
		{name: "active round ignored", usecaseErr: fmt.Errorf("wrapped: %w", multiplayer.ErrRoundIsStillActive)},
		{name: "unexpected failure ignored", usecaseErr: errors.New("database details")},
		{name: "reply fails", sendErr: errors.New("socket closed"), wantSends: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gameID := uuid.New()
			profile := user.MultiplayerUser{PublicProfile: user.PublicProfile{ID: uuid.New(), Username: "player"}}
			session := multiplayerSession(t, gameID.String(), profile)
			session.SendErr = tt.sendErr
			service := &wsfake.Service{}
			guesses := []multiplayer.Guess{{Username: profile.Username, RoundNum: 2, Score: 3456}}
			calls := 0
			uc := handlerUsecase{end: func(_ context.Context, req dto.EndMultiplayerRoundRequest) ([]multiplayer.Guess, error) {
				calls++

				assert.Equal(t, gameID, req.GameID)
				assert.Equal(t, profile.ID, req.UserID)
				assert.False(t, req.RequestTime.IsZero())

				return guesses, tt.usecaseErr
			}}
			handler.NewHandler(handler.Config{}, uc, nil, service)
			service.Message(session, transport.WebSocketMessageInput{Type: dto.MultiplayerMessageRoundEnd})
			assert.Equal(t, 1, calls)

			state := session.Snapshot()
			assert.Empty(t, state.Errors)
			assert.Empty(t, service.Broadcasts())

			if tt.wantSends == 0 {
				assert.Empty(t, state.Messages)
				return
			}

			assert.Equal(t, []transport.WebSocketMessageOutput{{
				Type: dto.MultiplayerMessageRoundFinished, Payload: map[string]any{"guesses": guesses},
			}}, state.Messages)
		})
	}
}

func TestMultiplayerDisconnectExcludesSender(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, room    string
		profile       any
		wantBroadcast bool
	}{
		{
			name: "admitted", room: "game", wantBroadcast: true,
			profile: user.MultiplayerUser{PublicProfile: user.PublicProfile{Username: "leaving"}},
		},
		{name: "missing room", profile: user.MultiplayerUser{PublicProfile: user.PublicProfile{Username: "leaving"}}},
		{name: "missing profile", room: "game"},
		{name: "wrong profile type", room: "game", profile: "wrong"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := multiplayerSession(t, tt.room, tt.profile)
			service := &wsfake.Service{}
			handler.NewHandler(handler.Config{}, nil, nil, service)
			service.Disconnect(session)

			if !tt.wantBroadcast {
				assert.Empty(t, service.Broadcasts())
				return
			}

			assert.Equal(t, []wsfake.Broadcast{{
				RoomID: "game", ExcludedSessionID: session.ID(),
				Message: transport.WebSocketMessageOutput{
					Type: dto.MultiplayerMessageUserDisconnected, Payload: map[string]any{"username": "leaving"},
				},
			}}, service.Broadcasts())
		})
	}
}

func TestMultiplayerWebSocketUpgradeDelegatesToTransport(t *testing.T) {
	t.Parallel()

	for _, transportErr := range []error{nil, errors.New("upgrade failed")} {
		name := "success"
		if transportErr != nil {
			name = "transport failure"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &wsfake.Service{RequestErr: transportErr}
			h := handler.NewHandler(handler.Config{}, nil, nil, service)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			h.HandleWS(httptest.NewRecorder(), req)
			require.Len(t, service.Requests(), 1)
			assert.Same(t, req, service.Requests()[0])
		})
	}
}

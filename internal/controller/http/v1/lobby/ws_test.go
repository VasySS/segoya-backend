package lobby_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/controller/http/v1/lobby"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

type settingsUsecase struct {
	lobby.Usecase

	update func(context.Context, dto.UpdateLobbySettingsRequest) error
}

func (u settingsUsecase) UpdateLobbySettings(ctx context.Context, req dto.UpdateLobbySettingsRequest) error {
	return u.update(ctx, req)
}

type settingsWebSocket struct {
	transport.WebSocketService

	messageHandler transport.WebSocketMessageHandler
	broadcasts     []transport.WebSocketMessageOutput
	broadcastIDs   []string
}

func (ws *settingsWebSocket) SetMessageHandler(handler transport.WebSocketMessageHandler) {
	ws.messageHandler = handler
}

func (*settingsWebSocket) SetConnectHandler(transport.WebSocketConnectHandler) {}

func (*settingsWebSocket) SetDisconnectHandler(transport.WebSocketDisconnectHandler) {}

func (ws *settingsWebSocket) Broadcast(id string, message transport.WebSocketMessageOutput) error {
	ws.broadcastIDs = append(ws.broadcastIDs, id)
	ws.broadcasts = append(ws.broadcasts, message)

	return nil
}

type settingsSession struct {
	transport.WebSocketSession

	request *http.Request
	profile user.PublicProfile
	errors  []string
}

func (s *settingsSession) Request() *http.Request {
	return s.request
}

func (s *settingsSession) Get(key string) (any, bool) {
	return s.profile, key == dto.LobbyUserProfileKey
}

func (*settingsSession) GetBroadcastID() (string, bool) {
	return "test-lobby", true
}

func (s *settingsSession) SendError(message string) {
	s.errors = append(s.errors, message)
}

func TestHandler_SettingsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings dto.LobbySettingsMessage
		wantErr  string
	}{
		{
			name:     "unsupported provider takes precedence over rounds and timer",
			settings: dto.LobbySettingsMessage{Provider: "unknown", Rounds: 0, TimerSeconds: 9},
			wantErr:  "provider field is set wrong",
		},
		{
			name:     "empty provider",
			settings: dto.LobbySettingsMessage{Provider: "", Rounds: 1, TimerSeconds: 10},
			wantErr:  "provider field is set wrong",
		},
		{
			name:     "rounds below minimum take precedence over timer",
			settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 0, TimerSeconds: 9},
			wantErr:  "rounds must be between 1 and 10",
		},
		{
			name:     "rounds above maximum",
			settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 11, TimerSeconds: 10},
			wantErr:  "rounds must be between 1 and 10",
		},
		{
			name:     "negative timer",
			settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 1, TimerSeconds: -1},
			wantErr:  "timer must be between 10 and 600",
		},
		{
			name:     "timer below minimum",
			settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 1, TimerSeconds: 9},
			wantErr:  "timer must be between 10 and 600",
		},
		{
			name:     "timer above maximum",
			settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 1, TimerSeconds: 601},
			wantErr:  "timer must be between 10 and 600",
		},
		{
			name:     "minimum rounds and timer",
			settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 1, TimerSeconds: 10},
		},
		{
			name: "maximum rounds and timer",
			settings: dto.LobbySettingsMessage{
				Provider: "yandex", Rounds: 10, TimerSeconds: 600, MovementAllowed: true,
			},
		},
		{
			name:     "untimed game",
			settings: dto.LobbySettingsMessage{Provider: "yandex_air", Rounds: 5, TimerSeconds: 0},
		},
		{
			name:     "seznam provider",
			settings: dto.LobbySettingsMessage{Provider: "seznam", Rounds: 5, TimerSeconds: 60},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ws := &settingsWebSocket{}
			profile := user.PublicProfile{ID: uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6943")}
			session := &settingsSession{
				request: httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), profile: profile,
			}

			var requests []dto.UpdateLobbySettingsRequest

			uc := settingsUsecase{update: func(_ context.Context, req dto.UpdateLobbySettingsRequest) error {
				requests = append(requests, req)
				return nil
			}}
			lobby.NewHandler(lobby.Config{}, uc, nil, ws)

			payload, err := json.Marshal(dto.LobbyNewSettingsMessage{Settings: tt.settings})
			require.NoError(t, err)

			ws.messageHandler(session, transport.WebSocketMessageInput{
				Type: dto.LobbyMessageTypeSettingsNew, Payload: payload,
			})

			if tt.wantErr != "" {
				assert.Equal(t, []string{tt.wantErr}, session.errors)
				assert.Empty(t, requests)
				assert.Empty(t, ws.broadcasts)

				return
			}

			assert.Empty(t, session.errors)
			require.Len(t, requests, 1)
			assert.Equal(t, "test-lobby", requests[0].LobbyID)
			assert.Equal(t, profile, requests[0].Creator)
			assert.Equal(t, tt.settings, requests[0].Settings)
			assert.False(t, requests[0].RequestTime.IsZero())
			assert.Equal(t, []string{"test-lobby"}, ws.broadcastIDs)
			assert.Equal(t, []transport.WebSocketMessageOutput{{
				Type: dto.LobbyMessageTypeSettingsChanged, Payload: map[string]any{"settings": tt.settings},
			}}, ws.broadcasts)
		})
	}
}

func TestHandler_SettingsFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		profile     user.PublicProfile
		payload     json.RawMessage
		wantCalls   int
		wantMessage string
	}{
		{
			name: "missing profile takes precedence over validation", payload: json.RawMessage(`{"settings":{}}`),
			wantMessage: "error getting user profile",
		},
		{
			name:        "malformed settings message",
			profile:     user.PublicProfile{Username: "creator"},
			payload:     json.RawMessage(`{"settings":`),
			wantMessage: "error unmarshalling settings message",
		},
		{
			name: "usecase error remains generic", profile: user.PublicProfile{Username: "creator"},
			payload:   json.RawMessage(`{"settings":{"provider":"google","rounds":1,"timerSeconds":10}}`),
			wantCalls: 1, wantMessage: "error updating lobby settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ws := &settingsWebSocket{}
			session := &settingsSession{
				request: httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), profile: tt.profile,
			}
			calls := 0
			uc := settingsUsecase{update: func(context.Context, dto.UpdateLobbySettingsRequest) error {
				calls++
				return errors.New("internal settings failure")
			}}
			lobby.NewHandler(lobby.Config{}, uc, nil, ws)

			ws.messageHandler(session, transport.WebSocketMessageInput{
				Type: dto.LobbyMessageTypeSettingsNew, Payload: tt.payload,
			})

			assert.Equal(t, []string{tt.wantMessage}, session.errors)
			assert.Equal(t, tt.wantCalls, calls)
			assert.Empty(t, ws.broadcasts)
		})
	}
}

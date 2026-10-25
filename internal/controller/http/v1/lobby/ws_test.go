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
	entitylobby "github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

type settingsUsecase struct {
	lobby.Usecase

	update func(context.Context, dto.UpdateLobbySettingsRequest) error
	get    func(context.Context, string) (entitylobby.Lobby, error)
}

func (u settingsUsecase) UpdateLobbySettings(ctx context.Context, req dto.UpdateLobbySettingsRequest) error {
	return u.update(ctx, req)
}

func (u settingsUsecase) GetLobby(ctx context.Context, id string) (entitylobby.Lobby, error) {
	return u.get(ctx, id)
}

func lobbySession(t *testing.T, room string, profile any) *wsfake.Session {
	t.Helper()
	session := &wsfake.Session{Req: httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)}
	session.SetBroadcastID(room)

	if profile != nil {
		session.Set(dto.LobbyUserProfileKey, profile)
	}

	return session
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

			ws := &wsfake.Service{}
			profile := user.PublicProfile{ID: uuid.MustParse("0199027e-ade6-784d-9d43-cc9c776a6943")}
			session := lobbySession(t, "test-lobby", profile)

			var requests []dto.UpdateLobbySettingsRequest

			uc := settingsUsecase{get: func(context.Context, string) (entitylobby.Lobby, error) {
				return entitylobby.Lobby{
					Provider:        tt.settings.Provider,
					Rounds:          tt.settings.Rounds,
					TimerSeconds:    tt.settings.TimerSeconds,
					MovementAllowed: tt.settings.MovementAllowed,
					PlayArea:        tt.settings.PlayArea,
				}, nil
			}, update: func(_ context.Context, req dto.UpdateLobbySettingsRequest) error {
				requests = append(requests, req)
				return nil
			}}
			lobby.NewHandler(lobby.Config{}, uc, nil, ws)

			payload, err := json.Marshal(dto.LobbyNewSettingsMessage{Settings: tt.settings})
			require.NoError(t, err)

			ws.Message(session, transport.WebSocketMessageInput{
				Type: dto.LobbyMessageTypeSettingsNew, Payload: payload,
			})

			if tt.wantErr != "" {
				assert.Equal(t, []string{tt.wantErr}, session.Snapshot().Errors)
				assert.Empty(t, requests)
				assert.Empty(t, ws.Broadcasts())

				return
			}

			assert.Empty(t, session.Snapshot().Errors)
			require.Len(t, requests, 1)
			assert.Equal(t, "test-lobby", requests[0].LobbyID)
			assert.Equal(t, profile, requests[0].Creator)
			assert.Equal(t, tt.settings, requests[0].Settings)
			assert.False(t, requests[0].RequestTime.IsZero())
			assert.Equal(t, []wsfake.Broadcast{{RoomID: "test-lobby", Message: transport.WebSocketMessageOutput{
				Type: dto.LobbyMessageTypeSettingsChanged, Payload: map[string]any{"settings": tt.settings},
			}}}, ws.Broadcasts())
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

			ws := &wsfake.Service{}
			session := lobbySession(t, "test-lobby", tt.profile)
			calls := 0
			uc := settingsUsecase{update: func(context.Context, dto.UpdateLobbySettingsRequest) error {
				calls++
				return errors.New("internal settings failure")
			}}
			lobby.NewHandler(lobby.Config{}, uc, nil, ws)

			ws.Message(session, transport.WebSocketMessageInput{
				Type: dto.LobbyMessageTypeSettingsNew, Payload: tt.payload,
			})

			assert.Equal(t, []string{tt.wantMessage}, session.Snapshot().Errors)
			assert.Equal(t, tt.wantCalls, calls)
			assert.Empty(t, ws.Broadcasts())
		})
	}
}

func TestHandlerBroadcastsResolvedArea(t *testing.T) {
	t.Parallel()

	profile := user.PublicProfile{ID: uuid.New()}
	settings := dto.LobbySettingsMessage{
		Provider: "google", Rounds: 3,
		PlayArea: playarea.Selection{Type: "saved", SavedID: uuid.New()},
	}
	polygon := playarea.Polygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {2, 0}, {0, 2}, {0, 0}}}}
	resolved := settings
	resolved.PlayArea = playarea.Selection{Type: "polygon", Polygon: &polygon}
	uc := settingsUsecase{
		update: func(_ context.Context, request dto.UpdateLobbySettingsRequest) error {
			require.Equal(t, settings, request.Settings)
			return nil
		},
		get: func(context.Context, string) (entitylobby.Lobby, error) {
			return entitylobby.Lobby{Provider: resolved.Provider, Rounds: resolved.Rounds, PlayArea: resolved.PlayArea}, nil
		},
	}
	ws := &wsfake.Service{}
	lobby.NewHandler(lobby.Config{}, uc, nil, ws)

	payload, err := json.Marshal(dto.LobbyNewSettingsMessage{Settings: settings})
	require.NoError(t, err)
	ws.Message(lobbySession(t, "room", profile), transport.WebSocketMessageInput{
		Type: dto.LobbyMessageTypeSettingsNew, Payload: payload,
	})
	require.Equal(t, []wsfake.Broadcast{{RoomID: "room", Message: transport.WebSocketMessageOutput{
		Type: dto.LobbyMessageTypeSettingsChanged, Payload: map[string]any{"settings": resolved},
	}}}, ws.Broadcasts())
}

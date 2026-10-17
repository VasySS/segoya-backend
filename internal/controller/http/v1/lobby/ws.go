package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

// getUser returns user profile from websocket session.
func getUser(s transport.WebSocketSession) (user.PublicProfile, bool) {
	userProfile, ok := s.Get(dto.LobbyUserProfileKey)
	if !ok {
		return user.PublicProfile{}, false
	}

	u, ok := userProfile.(user.PublicProfile)
	if !ok || u == (user.PublicProfile{}) {
		return user.PublicProfile{}, false
	}

	return u, true
}

// getLobbyUsers returns all connected users in the lobby using broadcast id (lobby id).
func (h Handler) getLobbyUsers(lobbyID string) []user.PublicProfile {
	sessions := h.ws.Sessions()
	users := make([]user.PublicProfile, 0)

	for _, s := range sessions {
		sLobbyID, ok := s.GetBroadcastID()
		if !ok || sLobbyID != lobbyID {
			continue
		}

		u, ok := getUser(s)
		if !ok {
			continue
		}

		users = append(users, u)
	}

	return users
}

// HandleWS upgrades http request to websocket.
func (h Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
	if err := h.ws.HandleRequest(w, r); err != nil {
		slog.Error("error handling ws request", slog.Any("error", err))
		return
	}
}

// lobbyAdmission keeps the reservation available even when initialization fails.
type lobbyAdmission struct {
	LobbyID   string
	UserID    uuid.UUID
	SessionID string
}

const lobbyAdmissionKey = "lobbyAdmission"

// handleWSConnect admits the socket before exposing it to lobby broadcasts.
func (h Handler) handleWSConnect(session transport.WebSocketSession) {
	req := session.Request()
	ctx := req.Context()
	lobbyID := chi.URLParam(req, "id")

	connected := false

	defer func() {
		if !connected {
			_ = session.Close()
		}
	}()

	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		session.SendError("error authorizing user")
		return
	}

	ticket := lobbyAdmission{LobbyID: lobbyID, UserID: claims.UserID, SessionID: session.ID()}
	session.Set(lobbyAdmissionKey, ticket)

	userProfile, err := h.uc.ConnectLobbyUser(ctx, lobbyID, claims.UserID, ticket.SessionID)
	if err != nil {
		// Admission may have succeeded even when the Valkey reply was lost.
		h.releaseAdmission(session, ticket)
		slog.Error("error connecting user to lobby", slog.Any("error", err))
		session.SendError("error connecting to lobby")

		return
	}

	if err := session.SendMessage(dto.LobbyMessageTypeConnectedUsers,
		map[string]any{"users": h.getLobbyUsers(lobbyID)}); err != nil {
		h.releaseAdmission(session, ticket)
		session.SendError("error connecting to lobby")

		return
	}

	session.Set(dto.LobbyUserProfileKey, userProfile)
	session.SetBroadcastID(lobbyID)

	connected = true
	_ = h.ws.Broadcast(lobbyID, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageTypeUserConnected,
		Payload: map[string]any{"user": userProfile},
	})
}

func (h Handler) releaseAdmission(session transport.WebSocketSession, ticket lobbyAdmission) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(session.Request().Context()), 5*time.Second)
	defer cancel()

	if err := h.uc.DisconnectLobbyUser(ctx, ticket.LobbyID, ticket.UserID, ticket.SessionID); err != nil {
		slog.Error("error releasing lobby admission", slog.Any("error", err))
	}
}

// handleWSMessage processes all incoming websocket messages from connected users.
func (h Handler) handleWSMessage(
	session transport.WebSocketSession,
	message transport.WebSocketMessageInput,
) {
	lobbyID, ok := session.GetBroadcastID()
	if !ok {
		session.SendError("lobby id not found")
		return
	}

	profile, admitted := getUser(session)
	if !admitted {
		session.SendError("error getting user profile")
		return
	}

	switch message.Type {
	case dto.LobbyMessageTypeChatInput:
		var chatInput dto.LobbyChatInputMessage
		if err := json.Unmarshal(message.Payload, &chatInput); err != nil {
			session.SendError("error unmarshalling chat message")
			return
		}

		chatInput.Message.Username = profile.Username
		h.processChatMsg(lobbyID, chatInput.Message)
	case dto.LobbyMessageTypeGameStart:
		h.processGameStart(session, lobbyID)
	case dto.LobbyMessageTypeSettingsNew:
		var settingsInput dto.LobbyNewSettingsMessage
		if err := json.Unmarshal(message.Payload, &settingsInput); err != nil {
			session.SendError("error unmarshalling settings message")
			return
		}

		h.processSettingsNew(session, lobbyID, settingsInput.Settings)
	default:
		slog.Debug("got unknown message type", slog.Any("type", message.Type))
	}
}

// handleWSDisconnect handles user disconnection from a lobby.
func (h Handler) handleWSDisconnect(session transport.WebSocketSession) {
	value, ok := session.Get(lobbyAdmissionKey)
	if !ok {
		return
	}

	ticket, ok := value.(lobbyAdmission)
	if !ok {
		return
	}

	h.releaseAdmission(session, ticket)

	lobbyID, ok := session.GetBroadcastID()
	if !ok {
		return
	}

	userProfile, ok := getUser(session)
	if !ok {
		return
	}

	_ = h.ws.BroadcastOthers(lobbyID, session, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageTypeUserDisconnected,
		Payload: map[string]any{"username": userProfile.Username},
	})
}

// processChatMsg handles incoming chat messages from users in the lobby.
func (h Handler) processChatMsg(
	lobbyID string,
	message dto.LobbyChatMessage,
) {
	message.Time = time.Now().UTC()

	_ = h.ws.Broadcast(lobbyID, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageTypeChatOutput,
		Payload: map[string]any{"message": message},
	})
}

// processGameStart initiates a start of a new game within the lobby.
func (h Handler) processGameStart(
	session transport.WebSocketSession,
	lobbyID string,
) {
	ctx := session.Request().Context()
	lobbyUsers := h.getLobbyUsers(lobbyID)

	creatorProfile, ok := getUser(session)
	if !ok {
		session.SendError("error getting user profile")
		return
	}

	gameID, err := h.uc.StartLobbyGame(ctx, dto.StartLobbyGameRequest{
		RequestTime:      time.Now().UTC(),
		LobbyID:          lobbyID,
		Creator:          creatorProfile,
		ConnectedPlayers: lobbyUsers,
	})
	if err != nil {
		slog.Error("error starting game", slog.Any("error", err))
		session.SendError("error starting game")

		return
	}

	_ = h.ws.Broadcast(lobbyID, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageTypeGameRedirect,
		Payload: map[string]any{"gameID": gameID.String()},
	})
}

// processSettingsNew handles incoming settings change messages from lobby creator.
func (h Handler) processSettingsNew(
	session transport.WebSocketSession,
	lobbyID string,
	settings dto.LobbySettingsMessage,
) {
	ctx := session.Request().Context()

	creatorProfile, ok := getUser(session)
	if !ok {
		session.SendError("error getting user profile")
		return
	}

	if err := settings.Validate(); err != nil {
		switch {
		case errors.Is(err, lobby.ErrInvalidProvider):
			session.SendError("provider field is set wrong")
		case errors.Is(err, lobby.ErrInvalidRounds):
			session.SendError("rounds must be between 1 and 10")
		case errors.Is(err, lobby.ErrInvalidTimerSeconds):
			session.SendError("timer must be between 10 and 600")
		}

		return
	}

	if err := h.uc.UpdateLobbySettings(ctx, dto.UpdateLobbySettingsRequest{
		RequestTime: time.Now().UTC(),
		LobbyID:     lobbyID,
		Creator:     creatorProfile,
		Settings:    settings,
	}); err != nil {
		slog.Error("error updating lobby settings", slog.Any("error", err))
		session.SendError("error updating lobby settings")

		return
	}

	_ = h.ws.Broadcast(lobbyID, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageTypeSettingsChanged,
		Payload: map[string]any{"settings": settings},
	})
}

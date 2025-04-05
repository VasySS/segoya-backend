package lobby

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	"github.com/go-chi/chi/v5"
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

// getLobbyUsers returns all connected users in the lobby.
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

// handleWSConnect handles a new websocket connection request.
func (h Handler) handleWSConnect(session transport.WebSocketSession) {
	req := session.Request()
	ctx := req.Context()
	lobbyID := chi.URLParam(req, "id")

	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		session.SendError("error authorizing user")
		return
	}

	userProfile, err := h.uc.ConnectLobbyUser(ctx, lobbyID, claims.UserID)
	if err != nil {
		slog.Error("error connecting user to lobby", slog.Any("error", err))
		session.SendError("error connecting to lobby")

		return
	}

	session.SetBroadcastID(lobbyID)
	session.Set(dto.LobbyUserProfileKey, userProfile)

	users := h.getLobbyUsers(lobbyID)

	if err := session.SendMessage(
		dto.LobbyMessageConnectedUsers,
		map[string]any{"users": users},
	); err != nil {
		slog.Error("error sending connected users", slog.Any("error", err))
		session.SendError("error connecting to lobby")

		return
	}

	_ = h.ws.BroadcastOthers(lobbyID, session, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageUserConnected,
		Payload: map[string]any{"user": userProfile},
	})
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

	switch message.Type {
	case dto.LobbyMessageChatInput:
		var chatInput dto.LobbyChatInputMessage
		if err := json.Unmarshal(message.Payload, &chatInput); err != nil {
			session.SendError("error unmarshalling msg")
			return
		}

		h.processChatMsg(session, lobbyID, chatInput.Message)
	case dto.LobbyMessageGameStart:
		h.processGameStart(session, lobbyID)
	case dto.LobbyMessageSettingsChanged:
	// h.processSettingsChanged(s, lobbyID)
	default:
		slog.Debug("got unknown message type", slog.Any("type", message.Type))
	}
}

// handleWSDisconnect handles user disconnection from a lobby.
func (h Handler) handleWSDisconnect(session transport.WebSocketSession) {
	req := session.Request()
	ctx := req.Context()

	lobbyID, ok := session.GetBroadcastID()
	if !ok {
		slog.Debug("error in lobby disconnect: lobby id not found in session")
		return
	}

	userProfile, ok := getUser(session)
	if !ok {
		slog.Debug("error in lobby disconnect: user not found in session")
		return
	}

	if err := h.uc.DisconnectLobbyUser(ctx, lobbyID, userProfile.ID); err != nil {
		slog.Debug("error disconnecting user from lobby", slog.Any("error", err))
		return
	}

	_ = h.ws.BroadcastOthers(lobbyID, session, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageUserDisconnected,
		Payload: map[string]any{"username": userProfile.Username},
	})
}

// processChatMsg handles incoming chat messages from users in the lobby.
func (h Handler) processChatMsg(
	session transport.WebSocketSession,
	lobbyID string,
	message dto.LobbyChatMessage,
) {
	_ = h.ws.BroadcastOthers(lobbyID, session, transport.WebSocketMessageOutput{
		Type:    dto.LobbyMessageChatOutput,
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
		Type:    dto.LobbyMessageGameRedirect,
		Payload: map[string]any{"gameID": gameID},
	})
}

// Package melody contains methods for working with Melody WebSocket library.
package melody

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	"github.com/olahol/melody"
)

var _ transport.WebSocketService = (*WebSocketService)(nil)

// WebSocketService is a wrapper around the Melody WebSocket library to handle WebSocket connections and messaging.
type WebSocketService struct {
	m *melody.Melody
}

// NewWebSocketService creates a new instance of WebSocketService with an initialized Melody WebSocket instance.
func NewWebSocketService() *WebSocketService {
	m := melody.New()

	return &WebSocketService{
		m: m,
	}
}

// Close closes the WebSocket service.
func (ws *WebSocketService) Close() error {
	if err := ws.m.Close(); err != nil {
		return fmt.Errorf("failed to close ws: %w", err)
	}

	return nil
}

// Sessions retrieves all active WebSocket sessions.
func (ws *WebSocketService) Sessions() []transport.WebSocketSession {
	res := make([]transport.WebSocketSession, 0)

	sessions, err := ws.m.Sessions()
	if err != nil {
		return res
	}

	for _, session := range sessions {
		res = append(res, NewSession(session))
	}

	return res
}

// Broadcast sends a message to all connected WebSocket clients.
func (ws *WebSocketService) Broadcast(
	broadcastID string,
	msg transport.WebSocketMessageOutput,
) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = ws.m.BroadcastFilter(msgBytes, func(session *melody.Session) bool {
		sessionReceiver := NewSession(session)

		receiverBroadcastID, ok := sessionReceiver.GetBroadcastID()
		if !ok {
			return false
		}

		return receiverBroadcastID == broadcastID
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	return nil
}

// BroadcastOthers sends a message to all connected WebSocket clients except the broadcaster.
func (ws *WebSocketService) BroadcastOthers(
	broadcastID string,
	broadcaster transport.WebSocketSession,
	msg transport.WebSocketMessageOutput,
) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = ws.m.BroadcastFilter(msgBytes, func(session *melody.Session) bool {
		sessionReceiver := NewSession(session)

		if sessionReceiver.ID() == broadcaster.ID() {
			return false
		}

		receiverBroadcastID, ok := sessionReceiver.GetBroadcastID()
		if !ok {
			return false
		}

		return receiverBroadcastID == broadcastID
	})
	if err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	return nil
}

// SetMessageHandler sets the handler function to process incoming WebSocket messages.
func (ws *WebSocketService) SetMessageHandler(handler transport.WebSocketMessageHandler) {
	ws.m.HandleMessage(func(session *melody.Session, msgBytes []byte) {
		var message transport.WebSocketMessageInput
		if err := json.Unmarshal(msgBytes, &message); err != nil {
			return
		}

		handler(NewSession(session), message)
	})
}

// SetConnectHandler sets the handler function to handle new WebSocket connections.
func (ws *WebSocketService) SetConnectHandler(handler transport.WebSocketConnectHandler) {
	ws.m.HandleConnect(func(session *melody.Session) {
		handler(NewSession(session))
	})
}

// SetDisconnectHandler sets the handler function to handle WebSocket disconnections.
func (ws *WebSocketService) SetDisconnectHandler(handler transport.WebSocketDisconnectHandler) {
	ws.m.HandleDisconnect(func(session *melody.Session) {
		handler(NewSession(session))
	})
}

// HandleRequest handles incoming HTTP requests to upgrade them to WebSocket connections.
func (ws *WebSocketService) HandleRequest(w http.ResponseWriter, r *http.Request) error {
	if err := ws.m.HandleRequest(w, r); err != nil {
		return fmt.Errorf("error handling ws request: %w", err)
	}

	return nil
}

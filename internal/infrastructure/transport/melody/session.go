package melody

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	"github.com/google/uuid"
	"github.com/olahol/melody"
)

const (
	webSocketIDKey          = "melody:id"
	webSocketBroadcastIDKey = "melody:broadcastID"
)

var _ transport.WebSocketSession = (*Session)(nil)

// Session is a wrapper for melody.Session.
type Session struct {
	ms *melody.Session
}

// NewSession creates a new instance of Session, wrapping a melody.Session.
func NewSession(s *melody.Session) *Session {
	return &Session{ms: s}
}

// ID returns the unique identifier for the session, creating a new one if necessary.
func (s *Session) ID() string {
	if id, ok := s.ms.Get(webSocketIDKey); ok {
		if val, ok := id.(string); ok {
			return val
		}
	}

	newID := uuid.New().String()
	s.Set(webSocketIDKey, newID)

	return newID
}

// Set stores a key-value pair in the session.
func (s *Session) Set(key string, value any) {
	s.ms.Set(key, value)
}

// SetBroadcastID sets the broadcast ID in the session.
func (s *Session) SetBroadcastID(id string) {
	s.Set(webSocketBroadcastIDKey, id)
}

// Get retrieves a value associated with a key from the session.
func (s *Session) Get(key string) (any, bool) {
	return s.ms.Get(key)
}

// GetBroadcastID retrieves the broadcast ID from the session if available.
func (s *Session) GetBroadcastID() (string, bool) {
	idVal, ok := s.Get(webSocketBroadcastIDKey)
	if !ok {
		return "", false
	}

	broadcastID, ok := idVal.(string)
	if !ok || broadcastID == "" {
		return "", false
	}

	return broadcastID, true
}

// Request returns the HTTP request associated with the WebSocket connection.
func (s *Session) Request() *http.Request {
	return s.ms.Request
}

// SendMessage sends a WebSocket message with the specified type and payload.
func (s *Session) SendMessage(typ transport.WebSocketMessageOutputType, payload map[string]any) error {
	msg := transport.WebSocketMessageOutput{
		Type:    typ,
		Payload: payload,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := s.ms.Write(msgBytes); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendError sends an error message through the WebSocket connection.
func (s *Session) SendError(text string) {
	msg := transport.NewWebsocketErrorMessage(text)

	msgBytes, _ := json.Marshal(msg) //nolint:errchkjson
	_ = s.ms.Write(msgBytes)
}

// Package transport contains interfaces and structs for working with WebSocket connections.
package transport

import (
	"encoding/json"
	"net/http"
)

type (
	// WebSocketMessageInputType is a message type for input messages.
	WebSocketMessageInputType string
	// WebSocketMessageInput is a message received from client.
	WebSocketMessageInput struct {
		Type    WebSocketMessageInputType `json:"type"`
		Payload json.RawMessage           `json:"payload"`
	}

	// WebSocketMessageOutputType is a message type for output messages.
	WebSocketMessageOutputType string
	// WebSocketMessageOutput is a message sent to client.
	WebSocketMessageOutput struct {
		Type    WebSocketMessageOutputType `json:"type"`
		Payload map[string]any             `json:"payload"`
	}

	// WebSocketSession is a session between client and server.
	WebSocketSession interface {
		ID() string
		Request() *http.Request
		Get(key string) (any, bool)
		GetBroadcastID() (string, bool)
		Set(key string, value any)
		SetBroadcastID(id string)
		SendMessage(typ WebSocketMessageOutputType, payload map[string]any) error
		SendError(message string)
	}

	// WebSocketMessageHandler is a handler for websocket messages.
	WebSocketMessageHandler func(session WebSocketSession, message WebSocketMessageInput)
	// WebSocketConnectHandler is a handler for when a client connects.
	WebSocketConnectHandler func(session WebSocketSession)
	// WebSocketDisconnectHandler is a handler for when a client disconnects.
	WebSocketDisconnectHandler func(session WebSocketSession)

	// WebSocketService is a service for handling websocket requests.
	WebSocketService interface {
		HandleRequest(w http.ResponseWriter, r *http.Request) error
		Sessions() []WebSocketSession
		Close() error
		Broadcast(id string, message WebSocketMessageOutput) error
		BroadcastOthers(id string, broadcaster WebSocketSession, message WebSocketMessageOutput) error
		SetMessageHandler(handler WebSocketMessageHandler)
		SetConnectHandler(handler WebSocketConnectHandler)
		SetDisconnectHandler(handler WebSocketDisconnectHandler)
	}
)

var (
	// WebSocketErrorMessage is a websocket error message type.
	WebSocketErrorMessage WebSocketMessageOutputType = "error" //nolint:gochecknoglobals
)

// NewWebsocketErrorMessage creates a new websocket error message.
func NewWebsocketErrorMessage(message string) WebSocketMessageOutput {
	return WebSocketMessageOutput{
		Type:    WebSocketErrorMessage,
		Payload: map[string]any{"message": message},
	}
}

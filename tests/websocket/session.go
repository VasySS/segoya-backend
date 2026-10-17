// Package websocket provides in-memory sessions for handler regression tests.
package websocket

import (
	"context"
	"net/http"

	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

type Tokens struct {
	Claims     user.AccessTokenClaims
	Authorized bool
}

func (t Tokens) FromContext(context.Context) (user.AccessTokenClaims, bool) {
	return t.Claims, t.Authorized
}

type Session struct {
	transport.WebSocketSession

	Req         *http.Request
	Values      map[string]any
	BroadcastID string
	Errors      []string
	Closed      bool
	SendErr     error
	BeforeSend  func()
}

func (s *Session) ID() string               { return "session" }
func (s *Session) Request() *http.Request   { return s.Req }
func (s *Session) Get(k string) (any, bool) { v, ok := s.Values[k]; return v, ok }
func (s *Session) Set(k string, v any) {
	if s.Values == nil {
		s.Values = make(map[string]any)
	}

	s.Values[k] = v
}
func (s *Session) GetBroadcastID() (string, bool) { return s.BroadcastID, s.BroadcastID != "" }
func (s *Session) SetBroadcastID(id string)       { s.BroadcastID = id }
func (s *Session) Close() error                   { s.Closed = true; return nil }
func (s *Session) SendError(msg string)           { s.Errors = append(s.Errors, msg) }
func (s *Session) SendMessage(transport.WebSocketMessageOutputType, map[string]any) error {
	if s.BeforeSend != nil {
		s.BeforeSend()
	}

	return s.SendErr
}

type Service struct {
	transport.WebSocketService

	Connect    transport.WebSocketConnectHandler
	Disconnect transport.WebSocketDisconnectHandler
	Message    transport.WebSocketMessageHandler
	Clients    []transport.WebSocketSession
	Broadcasts []transport.WebSocketMessageOutput
}

func (s *Service) SetConnectHandler(h transport.WebSocketConnectHandler)       { s.Connect = h }
func (s *Service) SetDisconnectHandler(h transport.WebSocketDisconnectHandler) { s.Disconnect = h }
func (s *Service) SetMessageHandler(h transport.WebSocketMessageHandler)       { s.Message = h }
func (s *Service) Sessions() []transport.WebSocketSession                      { return s.Clients }
func (s *Service) Broadcast(_ string, msg transport.WebSocketMessageOutput) error {
	s.Broadcasts = append(s.Broadcasts, msg)
	return nil
}

func (s *Service) BroadcastOthers(_ string, _ transport.WebSocketSession, msg transport.WebSocketMessageOutput) error {
	s.Broadcasts = append(s.Broadcasts, msg)
	return nil
}

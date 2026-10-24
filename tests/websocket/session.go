// Package websocket provides in-memory sessions for handler regression tests.
package websocket

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"sync"

	"github.com/google/uuid"

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

// Session configuration must be set before invoking handlers. Mutable state is
// accessed through methods so handlers and test observers can run concurrently.
type Session struct {
	Req        *http.Request
	SendErr    error
	BeforeSend func()

	once        sync.Once
	mu          sync.Mutex
	id          string
	values      map[string]any
	broadcastID string
	errors      []string
	closed      bool
	messages    []transport.WebSocketMessageOutput
}

type SessionSnapshot struct {
	BroadcastID string
	Values      map[string]any
	Errors      []string
	Closed      bool
	Messages    []transport.WebSocketMessageOutput
}

var _ transport.WebSocketSession = (*Session)(nil)

func (s *Session) ID() string {
	s.once.Do(func() { s.id = uuid.NewString() })
	return s.id
}

func (s *Session) Request() *http.Request { return s.Req }

func (s *Session) Get(k string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.values[k]

	return v, ok
}

func (s *Session) Set(k string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.values == nil {
		s.values = make(map[string]any)
	}

	s.values[k] = v
}

func (s *Session) GetBroadcastID() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.broadcastID, s.broadcastID != ""
}

func (s *Session) SetBroadcastID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.broadcastID = id
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	return nil
}

func (s *Session) SendError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.errors = append(s.errors, msg)
}

func (s *Session) SendMessage(typ transport.WebSocketMessageOutputType, payload map[string]any) error {
	if s.BeforeSend != nil {
		s.BeforeSend()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, transport.WebSocketMessageOutput{Type: typ, Payload: maps.Clone(payload)})

	return s.SendErr
}

// Snapshot copies recorded containers. Payload values are treated as immutable,
// just as values passed to a transport would be.
func (s *Session) Snapshot() SessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return SessionSnapshot{
		BroadcastID: s.broadcastID, Values: maps.Clone(s.values),
		Errors: slices.Clone(s.errors), Closed: s.closed, Messages: cloneMessages(s.messages),
	}
}

type Broadcast struct {
	RoomID            string
	ExcludedSessionID string
	Message           transport.WebSocketMessageOutput
}

// Service records transport calls without simulating delivery. Callbacks and
// configuration are registered before the service is used.
type Service struct {
	Connect    transport.WebSocketConnectHandler
	Disconnect transport.WebSocketDisconnectHandler
	Message    transport.WebSocketMessageHandler
	RequestErr error

	mu         sync.Mutex
	clients    []transport.WebSocketSession
	broadcasts []Broadcast
	requests   []*http.Request
}

var _ transport.WebSocketService = (*Service)(nil)

func (s *Service) SetConnectHandler(h transport.WebSocketConnectHandler)       { s.Connect = h }
func (s *Service) SetDisconnectHandler(h transport.WebSocketDisconnectHandler) { s.Disconnect = h }
func (s *Service) SetMessageHandler(h transport.WebSocketMessageHandler)       { s.Message = h }

func (s *Service) SetSessions(clients ...transport.WebSocketSession) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients = slices.Clone(clients)
}

func (s *Service) Sessions() []transport.WebSocketSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.clients)
}

func (s *Service) Broadcast(id string, msg transport.WebSocketMessageOutput) error {
	return s.recordBroadcast(id, "", msg)
}

func (s *Service) BroadcastOthers(
	id string, sender transport.WebSocketSession, msg transport.WebSocketMessageOutput,
) error {
	return s.recordBroadcast(id, sender.ID(), msg)
}

func (s *Service) Broadcasts() []Broadcast {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := slices.Clone(s.broadcasts)
	for i := range result {
		result[i].Message.Payload = maps.Clone(result[i].Message.Payload)
	}

	return result
}

func (s *Service) HandleRequest(_ http.ResponseWriter, r *http.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, r)

	return s.RequestErr
}

func (s *Service) Requests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.requests)
}

func (*Service) Close() error { return nil }

func (s *Service) recordBroadcast(id, excludedID string, msg transport.WebSocketMessageOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg.Payload = maps.Clone(msg.Payload)
	s.broadcasts = append(s.broadcasts, Broadcast{RoomID: id, ExcludedSessionID: excludedID, Message: msg})

	return nil
}

func cloneMessages(messages []transport.WebSocketMessageOutput) []transport.WebSocketMessageOutput {
	result := slices.Clone(messages)
	for i := range result {
		result[i].Payload = maps.Clone(result[i].Payload)
	}

	return result
}

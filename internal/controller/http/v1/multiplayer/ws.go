package multiplayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

// ErrGameIDNotFound is returned when the game id is not found in the session.
var ErrGameIDNotFound = errors.New("game id not found in session")

// getGameUser retrieves the multiplayer user profile from the websocket session.
func getGameUser(s transport.WebSocketSession) (user.MultiplayerUser, bool) {
	userProfile, ok := s.Get(dto.MultiplayerUserProfileKey)
	if !ok {
		return user.MultiplayerUser{}, false
	}

	u, ok := userProfile.(user.MultiplayerUser)
	if !ok || u == (user.MultiplayerUser{}) {
		return user.MultiplayerUser{}, false
	}

	return u, true
}

// getGameUsers retrieves the list of users currently connected to the game.
func (h Handler) getGameUsers(ctx context.Context, gameUUID uuid.UUID) ([]user.MultiplayerUser, error) {
	gameID := gameUUID.String()

	users, err := h.uc.GetGameUsers(ctx, gameUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game users: %w", err)
	}

	sessions := h.ws.Sessions()

	// check if user is connected
	for _, session := range sessions {
		if val, ok := session.GetBroadcastID(); !ok || val != gameID {
			continue
		}

		for i, u := range users {
			userInfo, ok := getGameUser(session)
			if ok && userInfo.ID == u.ID {
				users[i].Connected = true
			}
		}
	}

	return users, nil
}

// HandleWS handles the WebSocket connection initiation and upgrades the HTTP request to WebSocket.
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

	connected := false

	defer func() {
		if !connected {
			_ = session.Close()
		}
	}()

	gameID := chi.URLParam(req, "id")

	gameUUID, err := uuid.Parse(gameID)
	if err != nil {
		session.SendError("incorrect game id")
		return
	}

	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		session.SendError("error authorizing user")
		return
	}

	userProfile, err := h.uc.GetGameUser(ctx, claims.UserID, gameUUID)
	if err != nil {
		session.SendError("error connecting to game")
		return
	}

	gameUsers, err := h.getGameUsers(ctx, gameUUID)
	if err != nil {
		session.SendError("error getting game users")
		return
	}

	for _, u := range gameUsers {
		if u.ID == claims.UserID && u.Connected {
			session.SendError("a connection to this game already exists for user")
			return
		}
	}

	if err := session.SendMessage(dto.MultiplayerMessageConnectedUsers, map[string]any{"users": gameUsers}); err != nil {
		session.SendError("error getting game users")
		return
	}

	session.Set(dto.MultiplayerUserProfileKey, userProfile)
	session.SetBroadcastID(gameUUID.String())

	connected = true
	_ = h.ws.Broadcast(gameUUID.String(), transport.WebSocketMessageOutput{
		Type:    dto.MultiplayerMessageUserConnected,
		Payload: map[string]any{"user": userProfile},
	})
}

// handleWSMessage processes all incoming websocket messages from connected users.
func (h Handler) handleWSMessage(
	session transport.WebSocketSession,
	message transport.WebSocketMessageInput,
) {
	gameID, ok := session.GetBroadcastID()
	if !ok {
		session.SendError("game id not found")
		return
	}

	if _, admitted := getGameUser(session); !admitted {
		session.SendError("user is not admitted to game")
		return
	}

	switch message.Type {
	case dto.MultiplayerMessageUserGuess:
		var guess dto.MultiplayerUserGuessMessage
		if err := json.Unmarshal(message.Payload, &guess); err != nil {
			session.SendError("invalid guess: use guess:null or an object with valid lat and lng")
			return
		}

		h.processUserGuess(session, gameID, guess)
	case dto.MultiplayerMessageRoundEnd:
		h.processRoundEnd(session)
	default:
		session.SendError("unknown message type")
	}
}

// handleWSDisconnect handles user disconnection from a game.
func (h Handler) handleWSDisconnect(session transport.WebSocketSession) {
	gameID, ok := session.GetBroadcastID()
	if !ok {
		slog.Debug("error in game disconnect: game id not found in session")
		return
	}

	userProfile, ok := getGameUser(session)
	if !ok {
		slog.Debug("error in game disconnect: user not found in session")
		return
	}

	_ = h.ws.BroadcastOthers(gameID, session, transport.WebSocketMessageOutput{
		Type:    dto.MultiplayerMessageUserDisconnected,
		Payload: map[string]any{"username": userProfile.Username},
	})
}

// processUserGuess handles incoming user guess message.
func (h Handler) processUserGuess(
	session transport.WebSocketSession,
	gameID string,
	message dto.MultiplayerUserGuessMessage,
) {
	ctx := session.Request().Context()

	gameUUID, err := uuid.Parse(gameID)
	if err != nil {
		session.SendError("incorrect game id")
		return
	}

	userProfile, ok := getGameUser(session)
	if !ok {
		session.SendError("error getting user profile")
		return
	}

	err = h.uc.NewRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequest{
		RequestTime: time.Now().UTC(),
		GameID:      gameUUID,
		UserID:      userProfile.ID,
		Guess:       message.Guess,
	})

	switch {
	case errors.Is(err, game.ErrRoundNotStarted):
		session.SendError("round has not started")
		return
	case errors.Is(err, game.ErrInvalidCoordinates):
		session.SendError("invalid coordinates")
		return
	case errors.Is(err, multiplayer.ErrRoundAlreadyFinished):
		session.SendError("round already finished")
		slog.Debug("round already finished")

		return
	case err != nil:
		session.SendError("error saving guess")
		slog.Debug("error saving guess", slog.Any("error", err))

		return
	}

	_ = h.ws.Broadcast(gameID, transport.WebSocketMessageOutput{
		Type:    dto.MultiplayerMessageUserGuessed,
		Payload: map[string]any{"username": userProfile.Username},
	})
}

// processRoundEnd handles incoming round end message.
func (h Handler) processRoundEnd(
	session transport.WebSocketSession,
) {
	ctx := session.Request().Context()

	userProfile, ok := getGameUser(session)
	if !ok {
		session.SendError("error getting user profile")
		return
	}

	gameID, ok := session.GetBroadcastID()
	if !ok {
		session.SendError("game id not found")
		return
	}

	gameUUID, err := uuid.Parse(gameID)
	if err != nil {
		session.SendError("incorrect game id")
		return
	}

	guesses, err := h.uc.EndRound(ctx, dto.EndMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      gameUUID,
		UserID:      userProfile.ID,
	})

	switch {
	case errors.Is(err, multiplayer.ErrRoundIsStillActive):
		// TODO - just ignore?
		return
	case err != nil:
		slog.Error("error ending round (ws)", slog.Any("error", err))
		return
	}

	_ = session.SendMessage(dto.MultiplayerMessageRoundFinished, map[string]any{"guesses": guesses})
}

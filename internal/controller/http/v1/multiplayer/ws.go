package multiplayer

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	"github.com/go-chi/chi/v5"
)

// getUser retrieves the multiplayer user profile from the websocket session.
func getUser(s transport.WebSocketSession) (user.MultiplayerUser, bool) {
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
func (h Handler) getGameUsers(s transport.WebSocketSession) ([]user.MultiplayerUser, error) {
	ctx := s.Request().Context()

	gameID, ok := s.GetBroadcastID()
	if !ok {
		return nil, errors.New("game id not found in session")
	}

	gameIDInt, _ := strconv.Atoi(gameID)

	users, err := h.uc.GetGameUsers(ctx, gameIDInt)
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
			userInfo, ok := getUser(session)
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
	gameID := chi.URLParam(req, "id")

	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		session.SendError("error authorizing user")
		return
	}

	gameIDInt, err := strconv.Atoi(gameID)
	if err != nil {
		session.SendError("error parsing game id")
		return
	}

	userProfile, err := h.uc.GetGameUser(ctx, claims.UserID, gameIDInt)
	if err != nil {
		session.SendError("error connecting to game")
		return
	}

	session.SetBroadcastID(gameID)
	session.Set(dto.MultiplayerUserProfileKey, userProfile)

	gameUsers, err := h.getGameUsers(session)
	if err != nil {
		session.SendError("error getting game users")
		return
	}

	err = session.SendMessage(dto.MultiplayerMessageConnectedUsers, map[string]any{"users": gameUsers})
	if err != nil {
		session.SendError("error getting game users")
		return
	}

	_ = h.ws.BroadcastOthers(gameID, session, transport.WebSocketMessageOutput{
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

	switch message.Type {
	case dto.MultiplayerMessageUserGuess:
		var guess dto.MultiplayerUserGuessMessage
		if err := json.Unmarshal(message.Payload, &guess); err != nil {
			session.SendError("error unmarshalling msg")
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

	userProfile, ok := getUser(session)
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
	gameIDInt, _ := strconv.Atoi(gameID)

	userProfile, ok := getUser(session)
	if !ok {
		session.SendError("error getting user profile")
		return
	}

	err := h.uc.NewRoundGuess(ctx, dto.NewMultiplayerRoundGuessRequest{
		RequestTime: time.Now().UTC(),
		GameID:      gameIDInt,
		UserID:      userProfile.ID,
		Guess:       message.Guess,
	})
	if errors.Is(err, multiplayer.ErrRoundAlreadyFinished) {
		session.SendError("round already finished")
		slog.Debug("round already finished")

		return
	} else if err != nil {
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

	userProfile, ok := getUser(session)
	if !ok {
		session.SendError("error getting user profile")
		return
	}

	gameID, ok := session.GetBroadcastID()
	if !ok {
		session.SendError("game id not found")
		return
	}

	gameIDInt, _ := strconv.Atoi(gameID)

	guesses, err := h.uc.EndRound(ctx, dto.EndMultiplayerRoundRequest{
		RequestTime: time.Now().UTC(),
		GameID:      gameIDInt,
		UserID:      userProfile.ID,
	})
	if errors.Is(err, multiplayer.ErrRoundIsStillActive) {
		// TODO - just ignore?
		return
	} else if err != nil {
		slog.Error("error ending round (ws)", slog.Any("error", err))
		return
	}

	_ = session.SendMessage(dto.MultiplayerMessageRoundFinished, map[string]any{"guesses": guesses})
}

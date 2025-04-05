package valkey

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/valkey-io/valkey-go"
)

const (
	userPrefix             = "user:"
	sessionPrefix          = "session:"
	sessionIDField         = "sessionID"
	sessionUserIDField     = "userID"
	sessionTokenField      = "refreshToken"
	sessionUAField         = "ua"
	sessionLastActiveField = "lastActive"
)

// NewSession creates a new user session with specfied expiration time.
func (r *Repository) NewSession(ctx context.Context, req dto.NewSessionRequest) error {
	ctx, span := r.tracer.Start(ctx, "NewSession")
	defer span.End()

	key := userPrefix + strconv.Itoa(req.UserID) + sessionPrefix + req.SessionID

	fields := map[string]string{
		sessionIDField:         req.SessionID,
		sessionUserIDField:     strconv.Itoa(req.UserID),
		sessionTokenField:      req.RefreshToken,
		sessionUAField:         req.UA,
		sessionLastActiveField: req.RequestTime.Format(time.RFC3339),
	}

	setCmd := r.valkey.B().Hset().Key(key).FieldValue()
	for field, value := range fields {
		setCmd = setCmd.FieldValue(field, value)
	}

	expireCmd := r.valkey.B().Expire().Key(key).Seconds(int64(req.Expiration.Seconds()))

	cmds := make(valkey.Commands, 0, 2)
	cmds = append(cmds, setCmd.Build())
	cmds = append(cmds, expireCmd.Build())

	resp := r.valkey.DoMulti(ctx, cmds...)
	for _, resp := range resp {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("error creating user session: %w", err)
		}
	}

	return nil
}

// UpdateSession updates user session with new refresh token and last active time.
func (r *Repository) UpdateSession(ctx context.Context, req dto.UpdateSessionRequest) error {
	ctx, span := r.tracer.Start(ctx, "UpdateSession")
	defer span.End()

	key := userPrefix + strconv.Itoa(req.UserID) + sessionPrefix + req.SessionID

	setCmd := r.valkey.B().Hset().Key(key).FieldValue()
	setCmd = setCmd.FieldValue(sessionTokenField, req.RefreshToken)
	setCmd = setCmd.FieldValue(sessionLastActiveField, req.RequestTime.Format(time.RFC3339))

	expCmd := r.valkey.B().Expire().Key(key).Seconds(int64(req.Expiration.Seconds()))

	cmds := make(valkey.Commands, 0, 2)
	cmds = append(cmds, setCmd.Build())
	cmds = append(cmds, expCmd.Build())

	resp := r.valkey.DoMulti(ctx, cmds...)
	for _, resp := range resp {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("error refreshing user session: %w", err)
		}
	}

	return nil
}

// GetSession returns user session data.
func (r *Repository) GetSession(ctx context.Context, userID int, sessionID string) (user.Session, error) {
	ctx, span := r.tracer.Start(ctx, "GetSession")
	defer span.End()

	key := userPrefix + strconv.Itoa(userID) + sessionPrefix + sessionID
	cmd := r.valkey.B().Hgetall().Key(key).Build()

	resp, err := r.valkey.Do(ctx, cmd).AsStrMap()
	if err != nil {
		return user.Session{}, fmt.Errorf("failed to get session: %w", err)
	}

	if len(resp) == 0 {
		return user.Session{}, user.ErrSessionNotFound
	}

	return parseUserSessionData(resp)
}

// GetSessions returns all user sessions.
func (r *Repository) GetSessions(ctx context.Context, userID int) ([]user.Session, error) {
	ctx, span := r.tracer.Start(ctx, "GetSessions")
	defer span.End()

	key := userPrefix + strconv.Itoa(userID) + sessionPrefix
	cmd := r.valkey.B().Keys().Pattern(key + "*").Build()

	keys, err := r.valkey.Do(ctx, cmd).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions keys: %w", err)
	}

	sessionsCmds := make(valkey.Commands, 0, len(keys))
	for _, key := range keys {
		sessionsCmds = append(sessionsCmds, r.valkey.B().Hgetall().Key(key).Build())
	}

	results := r.valkey.DoMulti(ctx, sessionsCmds...)
	if results == nil {
		return []user.Session{}, nil
	}

	sessions := make([]user.Session, 0, len(results))

	for _, result := range results {
		resp, err := result.AsStrMap()
		if err != nil {
			slog.Debug("error getting session map", slog.Any("error", err))
			continue
		}

		if len(resp) == 0 {
			continue
		}

		session, err := parseUserSessionData(resp)
		if err != nil {
			slog.Debug("error parsing session data", slog.Any("error", err))
			continue
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteSession deletes user session.
func (r *Repository) DeleteSession(ctx context.Context, userID int, sessionID string) error {
	ctx, span := r.tracer.Start(ctx, "DeleteSession")
	defer span.End()

	key := userPrefix + strconv.Itoa(userID) + sessionPrefix + sessionID
	cmd := r.valkey.B().Del().Key(key).Build()

	if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("error deleting user session: %w", err)
	}

	return nil
}

func parseUserSessionData(data map[string]string) (user.Session, error) {
	userID, err := strconv.Atoi(data[sessionUserIDField])
	if err != nil {
		return user.Session{}, fmt.Errorf("invalid user_id: %w", err)
	}

	lastUsed, err := time.Parse(time.RFC3339, data[sessionLastActiveField])
	if err != nil {
		return user.Session{}, fmt.Errorf("invalid last_active: %w", err)
	}

	return user.Session{
		UserID:       userID,
		SessionID:    data[sessionIDField],
		RefreshToken: data[sessionTokenField],
		UA:           data[sessionUAField],
		LastActive:   lastUsed,
	}, nil
}

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// NewOAuth creates new oauth connection for user.
func (r *Repository) NewOAuth(ctx context.Context, req dto.NewOAuthRequestDB) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewOAuth")
	defer span.End()

	query := `
		INSERT INTO user_oauth
		(created_at, user_id, issuer, oauth_id)
		VALUES (@created_at, @user_id, @issuer, @oauth_id)
	`

	_, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"created_at": req.RequestTime,
		"user_id":    req.UserID,
		"issuer":     req.Issuer,
		"oauth_id":   req.OAuthID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return user.ErrOAuthAlreadyExists
		}

		return fmt.Errorf("failed to add oauth info: %w", err)
	}

	return nil
}

// GetOAuth returns all oauth connections for user.
func (r *Repository) GetOAuth(ctx context.Context, userID int) ([]user.OAuth, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetOAuth")
	defer span.End()

	query := `
		SELECT *
		FROM user_oauth
		WHERE user_id = @user_id
	`

	var providers []user.OAuth

	err := pgxscan.Select(ctx, tx, &providers, query, pgx.NamedArgs{"user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth info: %w", err)
	}

	return providers, nil
}

// DeleteOAuth deletes oauth connection for user.
func (r *Repository) DeleteOAuth(ctx context.Context, req dto.DeleteOAuthRequest) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "DeleteOAuth")
	defer span.End()

	query := `
		DELETE FROM user_oauth
		WHERE user_id = @user_id AND issuer = @issuer
	`

	_, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"user_id": req.UserID,
		"issuer":  req.Issuer,
	})
	if err != nil {
		return fmt.Errorf("failed to delete oauth info: %w", err)
	}

	return nil
}

// GetUserByOAuth returns user by oauth id.
func (r *Repository) GetUserByOAuth(
	ctx context.Context,
	req dto.GetUserByOAuthRequest,
) (user.PrivateProfile, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetUserByOAuth")
	defer span.End()

	query := `
		SELECT
			u.id,
			u.username,
			u.password,
			u.name,
			COALESCE(u.avatar_hash, '') AS avatar_hash,
			COALESCE(u.avatar_last_update, '0001-01-01') AS avatar_last_update,
			u.register_date
		FROM user_info AS u
		JOIN user_oauth AS o
			ON u.id = o.user_id
		WHERE o.oauth_id = @oauth_id AND o.issuer = @issuer
	`

	var u user.PrivateProfile

	err := pgxscan.Get(ctx, tx, &u, query, pgx.NamedArgs{
		"issuer":   req.Issuer,
		"oauth_id": req.OAuthID,
	})
	if pgxscan.NotFound(err) {
		return u, user.ErrOAuthNotFound
	} else if err != nil {
		return u, fmt.Errorf("failed to get user from db: %w", err)
	}

	return u, nil
}

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

// NewUser creates new user account.
func (r *Repository) NewUser(ctx context.Context, req dto.RegisterRequestDB) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewUser")
	defer span.End()

	query := `
		INSERT INTO user_info	
		(register_date, username, password, name) 
		VALUES (@register_date, @username, @password, @name)
	`

	_, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"register_date": req.RequestTime,
		"username":      req.Username,
		"password":      req.Password,
		"name":          req.Name,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return user.ErrAlreadyExists
		}

		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByUsername returns user's profile by username.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (user.PrivateProfile, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetUserByUsername")
	defer span.End()

	query := `
		SELECT 
			id,
			username,
			password,
			name,
			COALESCE(avatar_hash, '') AS avatar_hash,
			COALESCE(avatar_last_update, '0001-01-01') AS avatar_last_update,
			register_date
		FROM user_info
		WHERE username = @username
	`

	var u user.PrivateProfile

	err := pgxscan.Get(ctx, tx, &u, query, pgx.NamedArgs{"username": username})
	if pgxscan.NotFound(err) {
		return user.PrivateProfile{}, user.ErrUserNotFound
	} else if err != nil {
		return user.PrivateProfile{}, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

// GetUserByID returns user's profile by id.
func (r *Repository) GetUserByID(ctx context.Context, userID int) (user.PrivateProfile, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetUserByID")
	defer span.End()

	query := `
		SELECT
			id,
			username,
			password,
			name,
			COALESCE(avatar_hash, '') AS avatar_hash,
			COALESCE(avatar_last_update, '0001-01-01') AS avatar_last_update,
			register_date
		FROM user_info
		WHERE id = @id
	`

	var u user.PrivateProfile

	err := pgxscan.Get(ctx, tx, &u, query, pgx.NamedArgs{"id": userID})
	if pgxscan.NotFound(err) {
		return user.PrivateProfile{}, user.ErrUserNotFound
	} else if err != nil {
		return u, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

// UpdateAvatar updates user's avatar hash.
func (r *Repository) UpdateAvatar(ctx context.Context, req dto.UpdateAvatarRequestDB) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "UpdateAvatar")
	defer span.End()

	query := `
		UPDATE user_info
		SET 
			avatar_hash = @avatar_hash, 
			avatar_last_update = @update_time
		WHERE id = @user_id
	`

	_, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"avatar_hash": req.AvatarHash,
		"update_time": req.RequestTime,
		"user_id":     req.UserID,
	})
	if err != nil {
		return fmt.Errorf("failed to update user avatar: %w", err)
	}

	return nil
}

// UpdateUser updates user information.
func (r *Repository) UpdateUser(ctx context.Context, info dto.UpdateUserRequest) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "UpdateUser")
	defer span.End()

	query := `
		UPDATE user_info
		SET 
			name = @name 
		WHERE id = @id
	`

	_, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"id":   info.UserID,
		"name": info.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to update user info: %w", err)
	}

	return nil
}

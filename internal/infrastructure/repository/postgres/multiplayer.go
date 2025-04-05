package postgres

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// LockMultiplayerGame locks a multiplayer game exclusively.
func (r *Repository) LockMultiplayerGame(ctx context.Context, gameID int) error {
	tx := r.txManager.GetQueryEngine(ctx)

	query := `
		SELECT id
		FROM multiplayer_game
		WHERE id = @game_id
		FOR UPDATE
	`

	if _, err := tx.Exec(ctx, query, pgx.NamedArgs{"game_id": gameID}); err != nil {
		return fmt.Errorf("failed to lock game: %w", err)
	}

	return nil
}

// NewMultiplayerGame creates a new multiplayer game and returns its ID.
func (r *Repository) NewMultiplayerGame(
	ctx context.Context,
	req dto.NewMultiplayerGameRequest,
) (int, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewMultiplayerGame")
	defer span.End()

	query := `
        WITH 
		new_game AS (
            INSERT INTO multiplayer_game
                (created_at, creator_id, rounds, movement_allowed, provider, timer_seconds, players)
            VALUES (@created_at, @creator_id, @rounds, @movement_allowed, @provider, @timer_seconds, @players)
            RETURNING id
        ),
		inserted_users AS (
            INSERT INTO multiplayer_game_user (user_id, game_id, created_at)
            SELECT user_id, new_game.id, @created_at
            FROM new_game, unnest(@user_ids::bigint[]) AS user_id
        )
		
        SELECT id FROM new_game
    `

	userIDs := make([]int64, 0, len(req.ConnectedPlayers))
	for _, player := range req.ConnectedPlayers {
		userIDs = append(userIDs, int64(player.ID))
	}

	var gameID int

	err := pgxscan.Get(ctx, tx, &gameID, query, pgx.NamedArgs{
		"created_at":       req.RequestTime,
		"creator_id":       req.CreatorID,
		"rounds":           req.Rounds,
		"movement_allowed": req.MovementAllowed,
		"provider":         req.Provider,
		"timer_seconds":    req.TimerSeconds,
		"players":          len(req.ConnectedPlayers),
		"user_ids":         userIDs,
	})
	if err != nil {
		return -1, fmt.Errorf("failed to create multiplayer game: %w", err)
	}

	return gameID, nil
}

// GetMultiplayerGame returns a multiplayer game by its ID.
func (r *Repository) GetMultiplayerGame(ctx context.Context, gameID int) (multiplayer.Game, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetMultiplayerGame")
	defer span.End()

	query := `
		SELECT 
			mg.id,
			mg.creator_id,
			mg.rounds,
			COUNT(mr.id) AS round_current,
			mg.movement_allowed,
			mg.provider,
			mg.timer_seconds,
			mg.players,
			mg.finished,
			mg.created_at,
			COALESCE(mg.ended_at, '0001-01-01 00:00:00') AS ended_at
		FROM multiplayer_game AS mg
		LEFT JOIN multiplayer_round AS mr
			ON mr.game_id = mg.id
		WHERE mg.id = @game_id
		GROUP BY mg.id
	`

	var game multiplayer.Game

	err := pgxscan.Get(ctx, tx, &game, query, pgx.NamedArgs{"game_id": gameID})
	if pgxscan.NotFound(err) {
		return multiplayer.Game{}, multiplayer.ErrGameNotFound
	} else if err != nil {
		return multiplayer.Game{}, fmt.Errorf("failed to get multiplayer game by id: %w", err)
	}

	return game, nil
}

// EndMultiplayerGame ends a multiplayer game.
func (r *Repository) EndMultiplayerGame(ctx context.Context, req dto.EndMultiplayerGameRequestDB) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "EndMultiplayerGame")
	defer span.End()

	query := `
		UPDATE multiplayer_game
		SET 
			finished = true,
			ended_at = @ended_at
		WHERE id = @game_id
	`

	_, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"game_id":  req.GameID,
		"ended_at": req.RequestTime,
	})
	if err != nil {
		return fmt.Errorf("failed to end multiplayer game: %w", err)
	}

	return nil
}

// GetMultiplayerGameUser returns a multiplayer game user by its ID.
func (r *Repository) GetMultiplayerGameUser(ctx context.Context, userID, gameID int) (user.MultiplayerUser, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetMultiplayerGameUser")
	defer span.End()

	query := `
		SELECT 
			u.id, 
			u.name, 
			u.username,
			u.register_date,
			COALESCE(u.avatar_hash, '') AS avatar_hash,
			SUM(COALESCE(mru.score, 0)) AS score
		FROM user_info AS u 
		JOIN multiplayer_round AS mr
			ON mr.game_id = @game_id
		LEFT JOIN multiplayer_round_user AS mru 
			ON mru.round_id = mr.id
		WHERE u.id = @user_id
		GROUP BY u.id
	`

	var u user.MultiplayerUser

	err := pgxscan.Get(ctx, tx, &u, query, pgx.NamedArgs{
		"user_id": userID,
		"game_id": gameID,
	})
	if err != nil {
		return user.MultiplayerUser{}, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

// GetMultiplayerGameUsers returns a list of multiplayer game users.
func (r *Repository) GetMultiplayerGameUsers(ctx context.Context, gameID int) ([]user.MultiplayerUser, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetMultiplayerGameUsers")
	defer span.End()

	userQuery := `
		SELECT 
			u.id, 
			u.name, 
			u.username, 
			u.register_date,
			COALESCE(u.avatar_hash, '') AS avatar_hash,
			SUM(COALESCE(mru.score, 0)) AS score
		FROM multiplayer_game_user AS mgu
		JOIN user_info AS u
			ON u.id = mgu.user_id
		LEFT JOIN multiplayer_round_user AS mru 
			ON mru.user_id = mgu.user_id
		WHERE mgu.game_id = @game_id
		GROUP BY u.id
	`

	var users []user.MultiplayerUser

	err := pgxscan.Select(ctx, tx, &users, userQuery, pgx.NamedArgs{"game_id": gameID})
	if err != nil {
		return nil, fmt.Errorf("failed to get multiplayer game users: %w", err)
	}

	return users, nil
}

// GetMultiplayerGameGuesses returns a list of all multiplayer game guesses.
func (r *Repository) GetMultiplayerGameGuesses(ctx context.Context, gameID int) ([]multiplayer.Guess, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetMultiplayerGameGuesses")
	defer span.End()

	query := `
		SELECT 
			mr.round_num,
			pl.lat AS round_lat,
			pl.lng AS round_lng,
			u.username,
			COALESCE(u.avatar_hash, '') AS avatar_hash,
			mru.lat, 
			mru.lng, 
			mru.score
		FROM multiplayer_round_user AS mru
		JOIN multiplayer_round AS mr
			ON mr.id = mru.round_id
		JOIN panorama_location AS pl
			ON pl.id = mr.location_id
		JOIN user_info AS u 
			ON u.id = mru.user_id
		WHERE mr.game_id = @game_id
	`

	var guesses []multiplayer.Guess

	err := pgxscan.Select(ctx, tx, &guesses, query, pgx.NamedArgs{"game_id": gameID})
	if err != nil {
		return nil, fmt.Errorf("failed to get multiplayer user guesses: %w", err)
	}

	return guesses, nil
}

// NewMultiplayerRound creates a new multiplayer round.
func (r *Repository) NewMultiplayerRound(
	ctx context.Context,
	req dto.NewMultiplayerRoundRequestDB,
) (multiplayer.Round, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewMultiplayerRound")
	defer span.End()

	roundQuery := `
		INSERT INTO multiplayer_round
		(game_id, location_id, round_num, created_at, started_at)
		VALUES (@game_id, @location_id, @round_num, @created_at, @started_at)
		ON CONFLICT (game_id, round_num) DO NOTHING
		RETURNING id
	`

	var roundID int

	err := pgxscan.Get(ctx, tx, &roundID, roundQuery, pgx.NamedArgs{
		"game_id":     req.GameID,
		"location_id": req.LocationID,
		"round_num":   req.RoundNum,
		"created_at":  req.CreatedAt,
		"started_at":  req.StartedAt,
	})
	if err != nil {
		return multiplayer.Round{}, fmt.Errorf("failed to insert multiplayer round: %w", err)
	}

	round, err := r.GetMultiplayerRound(ctx, req.GameID, req.RoundNum)
	if err != nil {
		return multiplayer.Round{}, fmt.Errorf("failed to get multiplayer round: %w", err)
	}

	return round, nil
}

// GetMultiplayerRound returns a multiplayer round.
func (r *Repository) GetMultiplayerRound(ctx context.Context, gameID, roundNum int) (multiplayer.Round, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetMultiplayerRound")
	defer span.End()

	query := `
		SELECT 
			mr.id, 
			mr.game_id, 
			COALESCE(MAX(pl.streetview_id), '') AS streetview_id,
			MAX(pl.lat) AS lat,
    		MAX(pl.lng) AS lng, 
			COUNT(mru.id) AS guesses_count,
			mr.round_num, 
			mr.finished,
			mr.created_at, 
			mr.started_at,
			COALESCE(mr.ended_at, '0001-01-01 00:00:00') AS ended_at
		FROM multiplayer_round AS mr
		JOIN panorama_location AS pl
			ON pl.id = mr.location_id
		LEFT JOIN multiplayer_round_user AS mru
			ON mru.round_id = mr.id
		WHERE mr.game_id = @game_id AND mr.round_num = @round_num
		GROUP BY mr.id
	`

	var round multiplayer.Round

	err := pgxscan.Get(ctx, tx, &round, query, pgx.NamedArgs{
		"game_id":   gameID,
		"round_num": roundNum,
	})
	if pgxscan.NotFound(err) {
		return round, multiplayer.ErrRoundNotFound
	} else if err != nil {
		return round, fmt.Errorf("failed to get current multiplayer round: %w", err)
	}

	return round, nil
}

// EndMultiplayerRound ends a multiplayer round.
func (r *Repository) EndMultiplayerRound(ctx context.Context, req dto.EndMultiplayerRoundRequestDB) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "EndMultiplayerRound")
	defer span.End()

	roundQuery := `
		UPDATE multiplayer_round
		SET 
			finished = true,
			ended_at = @ended_at 
		WHERE id = @round_id
	`

	_, err := tx.Exec(ctx, roundQuery, pgx.NamedArgs{
		"round_id": req.RoundID,
		"ended_at": req.RequestTime,
	})
	if err != nil {
		return fmt.Errorf("failed to update round: %w", err)
	}

	return nil
}

// NewMultiplayerRoundGuess creates a new multiplayer round guess.
func (r *Repository) NewMultiplayerRoundGuess(
	ctx context.Context,
	req dto.NewMultiplayerRoundGuessRequestDB,
) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewMultiplayerRoundGuess")
	defer span.End()

	guessQuery := `
		INSERT INTO multiplayer_round_user
		(created_at, round_id, user_id, lat, lng, score, distance_miss_meters) 
		VALUES (@created_at, @round_id, @user_id, @lat, @lng, @score, @distance)
	`

	_, err := tx.Exec(ctx, guessQuery, pgx.NamedArgs{
		"created_at": req.RequestTime,
		"round_id":   req.RoundID,
		"user_id":    req.UserID,
		"lat":        req.Lat,
		"lng":        req.Lng,
		"score":      req.Score,
		"distance":   req.Distance,
	})
	if err != nil {
		return fmt.Errorf("failed to insert multiplayer user guess: %w", err)
	}

	return nil
}

// GetMultiplayerRoundGuesses returns a list of all multiplayer round guesses.
func (r *Repository) GetMultiplayerRoundGuesses(ctx context.Context, roundID int) ([]multiplayer.Guess, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetMultiplayerRoundGuesses")
	defer span.End()

	query := `
		SELECT 
			mr.round_num,
			pl.lat AS round_lat,
			pl.lng AS round_lng,
			u.username,
			COALESCE(u.avatar_hash, '') AS avatar_hash,
			mru.lat,
			mru.lng,
			mru.score
		FROM multiplayer_round AS mr
		JOIN panorama_location AS pl
			ON pl.id = mr.location_id
		JOIN multiplayer_game_user AS mgu
			ON mgu.game_id = mr.game_id
		JOIN multiplayer_round_user AS mru
			ON mru.user_id = mgu.user_id
		JOIN user_info AS u 
			ON u.id = mru.user_id
		WHERE mr.id = @round_id
	`

	var guesses []multiplayer.Guess

	err := pgxscan.Select(ctx, tx, &guesses, query, pgx.NamedArgs{"round_id": roundID})
	if err != nil {
		return nil, fmt.Errorf("failed to get multiplayer user guesses: %w", err)
	}

	return guesses, nil
}

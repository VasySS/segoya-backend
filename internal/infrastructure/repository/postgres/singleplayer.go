package postgres

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// LockSingleplayerGame locks singleplayer game by id exclusively.
func (r *Repository) LockSingleplayerGame(ctx context.Context, gameID int) error {
	tx := r.txManager.GetQueryEngine(ctx)

	query := `
		SELECT id
		FROM singleplayer_game
		WHERE id = @game_id
		FOR UPDATE
	`

	if _, err := tx.Exec(ctx, query, pgx.NamedArgs{"game_id": gameID}); err != nil {
		return fmt.Errorf("failed to lock game: %w", err)
	}

	return nil
}

// NewSingleplayerGame creates new singleplayer game.
func (r *Repository) NewSingleplayerGame(
	ctx context.Context,
	req dto.NewSingleplayerGameRequest,
) (int, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewSingleplayerGame")
	defer span.End()

	query := `
		INSERT INTO singleplayer_game
		(user_id, rounds, movement_allowed, provider, created_at, timer_seconds)
		VALUES (@user_id, @rounds, @movement_allowed, @provider, @created_at, @timer_seconds) 
		RETURNING id
	`

	var gameID int

	err := tx.QueryRow(ctx, query, pgx.NamedArgs{
		"user_id":          req.UserID,
		"rounds":           req.Rounds,
		"movement_allowed": req.MovementAllowed,
		"provider":         req.Provider,
		"created_at":       req.RequestTime,
		"timer_seconds":    req.TimerSeconds,
	}).Scan(&gameID)
	if err != nil {
		return -1, fmt.Errorf("failed to create singleplayer game: %w", err)
	}

	return gameID, nil
}

// GetSingleplayerGame returns singleplayer game by id.
func (r *Repository) GetSingleplayerGame(ctx context.Context, gameID int) (singleplayer.Game, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetSingleplayerGame")
	defer span.End()

	query := `
		SELECT 
			sg.id,
			sg.user_id,
			sg.rounds,
			COUNT(sr.id) AS round_current,
			sg.timer_seconds,
			sg.movement_allowed,
			sg.provider,
			COALESCE(SUM(srg.score), 0) AS score,
			sg.finished,
			sg.created_at,
			COALESCE(sg.ended_at, '0001-01-01 00:00:00') AS ended_at
		FROM singleplayer_game AS sg
		LEFT JOIN singleplayer_round AS sr
			ON sr.game_id = sg.id
		LEFT JOIN singleplayer_round_guess AS srg
			ON srg.round_id = sr.id
		WHERE sg.id = @game_id
		GROUP BY sg.id
	`

	var game singleplayer.Game

	err := pgxscan.Get(ctx, tx, &game, query, pgx.NamedArgs{"game_id": gameID})
	if pgxscan.NotFound(err) {
		return singleplayer.Game{}, singleplayer.ErrGameNotFound
	} else if err != nil {
		return singleplayer.Game{}, fmt.Errorf("failed to get singleplayer game: %w", err)
	}

	return game, nil
}

// GetSingleplayerGames returns a list of singleplayer games created by user.
func (r *Repository) GetSingleplayerGames(
	ctx context.Context,
	req dto.GetSingleplayerGamesRequest,
) ([]singleplayer.Game, int, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetSingleplayerGames")
	defer span.End()

	var totalGames int

	countQuery := `
		SELECT COUNT(*) 
		FROM singleplayer_game 
		WHERE user_id = @user_id
	`

	err := pgxscan.Get(ctx, tx, &totalGames, countQuery, pgx.NamedArgs{
		"user_id": req.UserID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total singleplayer games count: %w", err)
	}

	offset := (req.Page - 1) * req.PageSize
	query := `
		SELECT
			sg.id,
			sg.user_id,
			sg.rounds,
			COUNT(sr.id) AS round_current,
			sg.timer_seconds,
			sg.movement_allowed,
			sg.provider,
			COALESCE(SUM(srg.score), 0) AS score,
			sg.finished,
			sg.created_at,
			COALESCE(sg.ended_at, '0001-01-01 00:00:00') AS ended_at
		FROM singleplayer_game AS sg
		LEFT JOIN singleplayer_round AS sr
			ON sr.game_id = sg.id
		LEFT JOIN singleplayer_round_guess AS srg
			ON srg.round_id = sr.id
		WHERE user_id = @user_id
		GROUP BY sg.id
		ORDER BY created_at DESC
		LIMIT @limit OFFSET @offset
	`

	var games []singleplayer.Game

	err = pgxscan.Select(ctx, tx, &games, query, pgx.NamedArgs{
		"user_id": req.UserID,
		"limit":   req.PageSize,
		"offset":  offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get singleplayer games: %w", err)
	}

	return games, totalGames, nil
}

// EndSingleplayerGame finishes singleplayer game.
func (r *Repository) EndSingleplayerGame(ctx context.Context, req dto.EndSingleplayerGameRequestDB) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "EndSingleplayerGame")
	defer span.End()

	query := `
		UPDATE singleplayer_game
		SET 
			finished = true,
			ended_at = @ended_at
		WHERE id = @game_id
	`

	cmd, err := tx.Exec(ctx, query, pgx.NamedArgs{
		"game_id":  req.GameID,
		"ended_at": req.RequestTime,
	})
	if cmd.RowsAffected() == 0 {
		return singleplayer.ErrGameNotFound
	} else if err != nil {
		return fmt.Errorf("failed to finish singleplayer game: %w", err)
	}

	return nil
}

// NewSingleplayerRound creates new singleplayer round.
func (r *Repository) NewSingleplayerRound(
	ctx context.Context,
	req dto.NewSingleplayerRoundDBRequest,
) (singleplayer.Round, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewSingleplayerRound")
	defer span.End()

	roundQuery := `
		INSERT INTO singleplayer_round
		(game_id, location_id, created_at, started_at, round_num) 
		VALUES (@game_id, @location_id, @created_at, @started_at, @round_num)
		ON CONFLICT (game_id, round_num) DO NOTHING
		RETURNING id
	`

	var roundID int

	err := pgxscan.Get(ctx, tx, &roundID, roundQuery, pgx.NamedArgs{
		"game_id":     req.GameID,
		"location_id": req.LocationID,
		"created_at":  req.CreatedAt,
		"started_at":  req.StartedAt,
		"round_num":   req.RoundNum,
	})
	if err != nil {
		return singleplayer.Round{}, fmt.Errorf("failed to create singleplayer round: %w", err)
	}

	round, err := r.GetSingleplayerRoundByID(ctx, roundID)
	if err != nil {
		return singleplayer.Round{}, fmt.Errorf("failed to get singleplayer round: %w", err)
	}

	return round, nil
}

// GetSingleplayerRound returns a singleplayer round.
func (r *Repository) GetSingleplayerRound(ctx context.Context, gameID, roundNum int) (singleplayer.Round, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetSingleplayerRound")
	defer span.End()

	query := `
		SELECT 
			sr.id, 
			sr.game_id, 
			COALESCE(pl.streetview_id, '') AS streetview_id,
			pl.lat,
			pl.lng,
			sr.round_num, 
			sr.finished, 
			sr.created_at, 
			sr.started_at,
			COALESCE(sr.ended_at, '0001-01-01 00:00:00') AS ended_at
		FROM singleplayer_round AS sr
		JOIN panorama_location AS pl
			ON pl.id = sr.location_id
		WHERE sr.game_id = @game_id AND sr.round_num = @round_num
	`

	var round singleplayer.Round

	err := pgxscan.Get(ctx, tx, &round, query, pgx.NamedArgs{
		"game_id":   gameID,
		"round_num": roundNum,
	})
	if pgxscan.NotFound(err) {
		return singleplayer.Round{}, singleplayer.ErrRoundNotFound
	} else if err != nil {
		return round, fmt.Errorf("failed to get singleplayer round: %w", err)
	}

	return round, nil
}

// GetSingleplayerRoundByID returns a singleplayer round by its id.
func (r *Repository) GetSingleplayerRoundByID(ctx context.Context, roundID int) (singleplayer.Round, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetSingleplayerRoundByID")
	defer span.End()

	query := `
		SELECT 
			sr.id,
			sr.game_id,
			COALESCE(pl.streetview_id, '') AS streetview_id,
			pl.lat,
			pl.lng,
			sr.round_num,
			sr.finished,
			sr.created_at,
			sr.started_at,
			COALESCE(sr.ended_at, '0001-01-01 00:00:00') AS ended_at
		FROM singleplayer_round AS sr
		JOIN panorama_location AS pl
			ON pl.id = sr.location_id
		WHERE sr.id = @round_id
	`

	var round singleplayer.Round

	err := pgxscan.Get(ctx, tx, &round, query, pgx.NamedArgs{"round_id": roundID})
	if err != nil {
		return singleplayer.Round{}, fmt.Errorf("failed to get singleplayer round: %w", err)
	}

	return round, nil
}

// GetSingleplayerGameGuesses returns a list of guesses made during singleplayer game.
func (r *Repository) GetSingleplayerGameGuesses(
	ctx context.Context,
	gameID int,
) ([]singleplayer.Guess, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetSingleplayerGameGuesses")
	defer span.End()

	query := `
		SELECT 
			sr.round_num,
			pl.lat AS round_lat,
			pl.lng AS round_lng,
			srg.lat AS guess_lat, 
			srg.lng AS guess_lng, 
			srg.score, 
			srg.distance_miss_meters AS miss_distance
		FROM singleplayer_round AS sr 
		JOIN panorama_location AS pl 
			ON pl.id = sr.location_id
		JOIN singleplayer_round_guess AS srg
			ON srg.round_id = sr.id
		WHERE sr.game_id = @game_id
	`

	var rounds []singleplayer.Guess

	err := pgxscan.Select(ctx, tx, &rounds, query, pgx.NamedArgs{"game_id": gameID})
	if pgxscan.NotFound(err) {
		return nil, singleplayer.ErrGameNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to get singleplayer rounds: %w", err)
	}

	return rounds, nil
}

// NewSingleplayerRoundGuess creates a new guess for singleplayer round.
func (r *Repository) NewSingleplayerRoundGuess(
	ctx context.Context,
	req dto.NewSingleplayerRoundGuessRequest,
) error {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "NewSingleplayerRoundGuess")
	defer span.End()

	// Using CTE to perform both INSERT and UPDATE
	combinedQuery := `
		WITH 
		insert_guess AS (
			INSERT INTO singleplayer_round_guess
			(round_id, created_at, lat, lng, score, distance_miss_meters)
			VALUES (@round_id, @created_at, @lat, @lng, @score, @distance)
		)
		
		UPDATE singleplayer_round
		SET 
			finished = true,
			ended_at = @ended_at
		WHERE id = @round_id
	`

	_, err := tx.Exec(ctx, combinedQuery, pgx.NamedArgs{
		"round_id":   req.RoundID,
		"lat":        req.Guess.Lat,
		"lng":        req.Guess.Lng,
		"score":      req.Score,
		"distance":   req.Distance,
		"created_at": req.RequestTime,
		"ended_at":   req.RequestTime,
	})
	if err != nil {
		return fmt.Errorf("failed to set singleplayer guess and update round: %w", err)
	}

	return nil
}

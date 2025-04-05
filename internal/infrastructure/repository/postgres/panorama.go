package postgres

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// RandomGoogleStreetview gets a random Google streetview from the database.
func (r *Repository) RandomGoogleStreetview(ctx context.Context) (game.GoogleStreetview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "RandomGoogleStreetview")
	defer span.End()

	query := `
		SELECT
			id,
			lat, 
			lng
		FROM panorama_location
		WHERE provider = 'google'
		ORDER BY RANDOM() 
		LIMIT 1
	`

	var stv game.GoogleStreetview

	err := pgxscan.Get(ctx, tx, &stv, query)
	if err != nil {
		return game.GoogleStreetview{}, fmt.Errorf("failed to get random google streetview: %w", err)
	}

	return stv, nil
}

// GetGoogleStreetview gets a Google streetview by id from the database.
func (r *Repository) GetGoogleStreetview(ctx context.Context, id int) (game.GoogleStreetview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetGoogleStreetview")
	defer span.End()

	query := `
		SELECT 
			id,
			lat, 
			lng
		FROM panorama_location
		WHERE provider = 'google' AND id = @location_id
	`

	var stv game.GoogleStreetview

	err := pgxscan.Get(ctx, tx, &stv, query, pgx.NamedArgs{"location_id": id})
	if err != nil {
		return game.GoogleStreetview{}, fmt.Errorf("failed to get streetview by id: %w", err)
	}

	return stv, nil
}

// RandomYandexAirview gets a random Yandex air view from the database.
func (r *Repository) RandomYandexAirview(ctx context.Context) (game.YandexAirview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "RandomYandexAirview")
	defer span.End()

	query := `
		SELECT 
			id,
			COALESCE(streetview_id, '') AS streetview_id,
			lat, 
			lng
		FROM panorama_location
		WHERE provider = 'yandex_air'
		ORDER BY random() 
		LIMIT 1
	`

	var airv game.YandexAirview

	err := pgxscan.Get(ctx, tx, &airv, query)
	if err != nil {
		return game.YandexAirview{}, fmt.Errorf("failed to get random air view: %w", err)
	}

	return airv, nil
}

// GetYandexAirview gets a Yandex air view by id from the database.
func (r *Repository) GetYandexAirview(ctx context.Context, id int) (game.YandexAirview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetYandexAirview")
	defer span.End()

	query := `
		SELECT 
			id,
			COALESCE(streetview_id, '') AS streetview_id,
			lat, 
			lng
		FROM panorama_location
		WHERE provider = 'yandex_air' AND id = @location_id
	`

	var airv game.YandexAirview

	err := pgxscan.Get(ctx, tx, &airv, query, pgx.NamedArgs{"location_id": id})
	if err != nil {
		return game.YandexAirview{}, fmt.Errorf("failed to get air view by id: %w", err)
	}

	return airv, nil
}

// RandomYandexStreetview gets a random Yandex streetview from the database.
func (r *Repository) RandomYandexStreetview(ctx context.Context) (game.YandexStreetview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "RandomYandexStreetview")
	defer span.End()

	query := `
		SELECT 
			id,
			lat, 
			lng
		FROM panorama_location AS pl
		WHERE provider = 'yandex'
		ORDER BY random() 
		LIMIT 1
	`

	var stv game.YandexStreetview

	err := pgxscan.Get(ctx, tx, &stv, query)
	if err != nil {
		return game.YandexStreetview{}, fmt.Errorf("failed to get random streetview: %w", err)
	}

	return stv, nil
}

// GetYandexStreetview gets a Yandex streetview by id from the database.
func (r *Repository) GetYandexStreetview(ctx context.Context, id int) (game.YandexStreetview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetYandexStreetview")
	defer span.End()

	query := `
		SELECT
			id,
			lat, 
			lng
		FROM panorama_location AS pl
		WHERE provider = 'yandex' AND id = @id
	`

	var stv game.YandexStreetview

	err := pgxscan.Get(ctx, tx, &stv, query, pgx.NamedArgs{"id": id})
	if err != nil {
		return game.YandexStreetview{}, fmt.Errorf("failed to get streetview by id: %w", err)
	}

	return stv, nil
}

// RandomSeznamStreetview gets a random Seznam streetview from the database.
func (r *Repository) RandomSeznamStreetview(ctx context.Context) (game.SeznamStreetview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "RandomSeznamStreetview")
	defer span.End()

	query := `
		SELECT 
			id,
			lat, 
			lng
		FROM panorama_location
		WHERE provider = 'seznam'
		ORDER BY random() 
		LIMIT 1
	`

	var stv game.SeznamStreetview

	err := pgxscan.Get(ctx, tx, &stv, query)
	if err != nil {
		return stv, fmt.Errorf("failed to get random streetview point: %w", err)
	}

	return stv, nil
}

// GetSeznamStreetview gets a Seznam streetview by id from the database.
func (r *Repository) GetSeznamStreetview(ctx context.Context, id int) (game.SeznamStreetview, error) {
	tx := r.txManager.GetQueryEngine(ctx)

	ctx, span := r.tracer.Start(ctx, "GetSeznamStreetview")
	defer span.End()

	query := `
		SELECT 
			id,
			lat, 
			lng
		FROM panorama_location
		WHERE provider = 'seznam' AND id = @id
	`

	var stv game.SeznamStreetview

	err := pgxscan.Get(ctx, tx, &stv, query, pgx.NamedArgs{"id": id})
	if err != nil {
		return stv, fmt.Errorf("failed to get streetview point by id: %w", err)
	}

	return stv, nil
}

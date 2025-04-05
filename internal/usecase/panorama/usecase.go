// Package panorama provides panorama metadata for singleplayer and multiplayer games.
package panorama

import (
	"context"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Repository provides access to panorama metadata.
//
//go:generate go tool mockery --name=Repository
type Repository interface {
	GetGoogleStreetview(ctx context.Context, id int) (game.GoogleStreetview, error)
	GetSeznamStreetview(ctx context.Context, id int) (game.SeznamStreetview, error)
	GetYandexStreetview(ctx context.Context, id int) (game.YandexStreetview, error)
	GetYandexAirview(ctx context.Context, id int) (game.YandexAirview, error)
	RandomGoogleStreetview(ctx context.Context) (game.GoogleStreetview, error)
	RandomSeznamStreetview(ctx context.Context) (game.SeznamStreetview, error)
	RandomYandexAirview(ctx context.Context) (game.YandexAirview, error)
	RandomYandexStreetview(ctx context.Context) (game.YandexStreetview, error)
}

// Usecase contains business logic for panorama metadata management.
type Usecase struct {
	cfg    Config
	repo   Repository
	tracer trace.Tracer
}

// NewUsecase creates and returns a new Usecase instance with the provided dependencies.
//
// cfg - Configuration settings for the Usecase.
//
// repo - Implementation of the Repository interface for accessing panorama metadata.
func NewUsecase(cfg Config, repo Repository) *Usecase {
	return &Usecase{
		cfg:    cfg,
		repo:   repo,
		tracer: otel.GetTracerProvider().Tracer("PanoramaUsecase"),
	}
}

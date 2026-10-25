// Package playarea resolves geographic selections and manages saved custom areas.
package playarea

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	area "github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// Repository provides spatial queries and access-controlled saved areas.
type Repository interface {
	CountPlayAreaLocations(ctx context.Context, provider game.PanoramaProvider, selection area.Selection) (int, error)
	ValidatePlayAreaPresets(ctx context.Context, provider game.PanoramaProvider, ids []uuid.UUID) error
	ListPlayAreaPresets(ctx context.Context, provider game.PanoramaProvider) ([]area.Preset, error)
	GetPlayAreaPreset(ctx context.Context, id uuid.UUID) (area.Preset, error)
	CreatePlayArea(
		ctx context.Context,
		owner uuid.UUID,
		name string,
		polygon area.Polygon,
		visibility string,
	) (area.Saved, error)
	GetPlayArea(ctx context.Context, id, user uuid.UUID) (area.Saved, error)
	ListPlayAreas(ctx context.Context, owner uuid.UUID, page, pageSize int) ([]area.Saved, int, error)
	UpdatePlayArea(
		ctx context.Context,
		id, owner uuid.UUID,
		name *string,
		polygon *area.Polygon,
		visibility *string,
	) (area.Saved, error)
	DeletePlayArea(ctx context.Context, id, owner uuid.UUID) error
}

// Usecase applies validation and creates immutable selections from saved geometry.
type Usecase struct {
	repo   Repository
	tracer trace.Tracer
}

func NewUsecase(repo Repository) *Usecase {
	return &Usecase{repo: repo, tracer: otel.Tracer("PlayAreaUsecase")}
}

func (uc *Usecase) Resolve(
	ctx context.Context,
	provider game.PanoramaProvider,
	s area.Selection,
	userID uuid.UUID,
) (area.Selection, error) {
	if !provider.IsSupported() {
		return area.Selection{}, game.ErrInvalidProvider
	}

	s, err := s.Normalize()
	if err != nil {
		return area.Selection{}, fmt.Errorf("validate play area selection: %w", err)
	}

	if s.Type == "saved" {
		saved, err := uc.repo.GetPlayArea(ctx, s.SavedID, userID)
		if err != nil {
			return area.Selection{}, fmt.Errorf("resolve saved play area: %w", err)
		}

		s = area.Selection{Type: "polygon", Polygon: &saved.Polygon}

		resolved, err := s.Normalize()
		if err != nil {
			return area.Selection{}, fmt.Errorf("validate saved polygon: %w", err)
		}

		return resolved, nil
	}

	if s.Type == "presets" {
		if err := uc.repo.ValidatePlayAreaPresets(ctx, provider, s.PresetIDs); err != nil {
			return area.Selection{}, fmt.Errorf("validate area presets: %w", err)
		}
	}

	return s, nil
}

// Count reports exact provider-specific counts. No location coordinates leave this usecase.
func (uc *Usecase) Count(
	ctx context.Context,
	provider game.PanoramaProvider,
	s area.Selection,
	userID uuid.UUID,
) (int, error) {
	ctx, span := uc.tracer.Start(ctx, "CountPlayAreaLocations")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	s, err := uc.Resolve(ctx, provider, s, userID)
	if err != nil {
		return 0, err
	}

	n, err := uc.repo.CountPlayAreaLocations(ctx, provider, s)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("count area locations: %w", err)
	}

	return n, nil
}

// Prepare rechecks capacity immediately before creating a game, within its transaction.
func (uc *Usecase) Prepare(
	ctx context.Context,
	provider game.PanoramaProvider,
	s area.Selection,
	userID uuid.UUID,
	rounds int,
) (area.Selection, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	s, err := uc.Resolve(ctx, provider, s, userID)
	if err != nil {
		return area.Selection{}, err
	}

	n, err := uc.repo.CountPlayAreaLocations(ctx, provider, s)
	if err != nil {
		return area.Selection{}, fmt.Errorf("count game locations: %w", err)
	}

	if n < rounds {
		return area.Selection{}, fmt.Errorf("%w: need %d, available %d", area.ErrInsufficientLocations, rounds, n)
	}

	return s, nil
}

func (uc *Usecase) Presets(ctx context.Context, provider game.PanoramaProvider) ([]area.Preset, error) {
	if !provider.IsSupported() {
		return nil, game.ErrInvalidProvider
	}

	presets, err := uc.repo.ListPlayAreaPresets(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("list area presets: %w", err)
	}

	return presets, nil
}

func (uc *Usecase) Preset(ctx context.Context, id uuid.UUID) (area.Preset, error) {
	preset, err := uc.repo.GetPlayAreaPreset(ctx, id)
	if err != nil {
		return area.Preset{}, fmt.Errorf("get area preset: %w", err)
	}

	return preset, nil
}

func (uc *Usecase) Create(
	ctx context.Context,
	owner uuid.UUID,
	name string,
	polygon area.Polygon,
	visibility string,
) (area.Saved, error) {
	if visibility == "" {
		visibility = "private"
	}

	if err := area.ValidateName(name); err != nil {
		return area.Saved{}, fmt.Errorf("validate saved play area: %w", err)
	}

	if err := area.ValidateVisibility(visibility); err != nil {
		return area.Saved{}, fmt.Errorf("validate saved play area: %w", err)
	}

	if err := polygon.Validate(); err != nil {
		return area.Saved{}, fmt.Errorf("validate saved play area: %w", err)
	}

	saved, err := uc.repo.CreatePlayArea(ctx, owner, strings.TrimSpace(name), polygon, visibility)
	if err != nil {
		return area.Saved{}, fmt.Errorf("create play area: %w", err)
	}

	return saved, nil
}

func (uc *Usecase) Get(ctx context.Context, id, userID uuid.UUID) (area.Saved, error) {
	saved, err := uc.repo.GetPlayArea(ctx, id, userID)
	if err != nil {
		return area.Saved{}, fmt.Errorf("get play area: %w", err)
	}

	return saved, nil
}

func (uc *Usecase) List(ctx context.Context, owner uuid.UUID, page, pageSize int) ([]area.Saved, int, error) {
	if page < 1 || pageSize < 1 || pageSize > 50 {
		return nil, 0, area.ErrInvalidSelection
	}

	saved, total, err := uc.repo.ListPlayAreas(ctx, owner, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list play areas: %w", err)
	}

	return saved, total, nil
}

func (uc *Usecase) Update(
	ctx context.Context,
	id, owner uuid.UUID,
	name *string,
	polygon *area.Polygon,
	visibility *string,
) (area.Saved, error) {
	if name != nil {
		if err := area.ValidateName(*name); err != nil {
			return area.Saved{}, fmt.Errorf("validate saved play area: %w", err)
		}

		trimmed := strings.TrimSpace(*name)
		name = &trimmed
	}

	if polygon != nil {
		if err := polygon.Validate(); err != nil {
			return area.Saved{}, fmt.Errorf("validate saved play area: %w", err)
		}
	}

	if visibility != nil {
		if err := area.ValidateVisibility(*visibility); err != nil {
			return area.Saved{}, fmt.Errorf("validate saved play area: %w", err)
		}
	}

	saved, err := uc.repo.UpdatePlayArea(ctx, id, owner, name, polygon, visibility)
	if err != nil {
		return area.Saved{}, fmt.Errorf("update play area: %w", err)
	}

	return saved, nil
}

func (uc *Usecase) Delete(ctx context.Context, id, owner uuid.UUID) error {
	if err := uc.repo.DeletePlayArea(ctx, id, owner); err != nil {
		return fmt.Errorf("delete play area: %w", err)
	}

	return nil
}

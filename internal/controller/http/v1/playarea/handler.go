// Package playarea implements authenticated geographic area endpoints.
package playarea

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	area "github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// Usecase is the area-management boundary consumed by HTTP handlers.
type Usecase interface {
	Count(ctx context.Context, provider game.PanoramaProvider, selection area.Selection, user uuid.UUID) (int, error)
	Presets(ctx context.Context, provider game.PanoramaProvider) ([]area.Preset, error)
	Preset(ctx context.Context, id uuid.UUID) (area.Preset, error)
	Create(ctx context.Context, owner uuid.UUID, name string, polygon area.Polygon, visibility string) (area.Saved, error)
	Get(ctx context.Context, id, user uuid.UUID) (area.Saved, error)
	List(ctx context.Context, owner uuid.UUID, page, pageSize int) ([]area.Saved, int, error)
	Update(
		ctx context.Context,
		id, owner uuid.UUID,
		name *string,
		polygon *area.Polygon,
		visibility *string,
	) (area.Saved, error)
	Delete(ctx context.Context, id, owner uuid.UUID) error
}
type TokenService interface {
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
}
type Handler struct {
	uc Usecase
	ts TokenService
}

var (
	_               api.PlayAreasHandler = (*Handler)(nil)
	errUnauthorized                      = errors.New("authentication required")
)

func NewHandler(uc Usecase, ts TokenService) *Handler { return &Handler{uc: uc, ts: ts} }

func problem(err error) *api.BackendError {
	status := http.StatusInternalServerError
	detail := "An error occurred while processing the play area"

	switch {
	case errors.Is(err, errUnauthorized):
		status = http.StatusUnauthorized
		detail = err.Error()
	case errors.Is(err, area.ErrNotFound):
		status = http.StatusNotFound
		detail = area.ErrNotFound.Error()
	case area.IsInvalid(err) || errors.Is(err, game.ErrInvalidProvider):
		status = http.StatusBadRequest
		detail = err.Error()
	default:
		slog.Error("play area operation failed", slog.Any("error", err))
	}

	return &api.BackendError{Title: http.StatusText(status), Status: status, Detail: detail}
}

func (h Handler) CountPlayArea(ctx context.Context, req *api.CountPlayAreaRequest) (api.CountPlayAreaRes, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return countPlayAreaError(err), nil
	}

	selection, err := dto.PlayAreaFromAPI(api.NewOptPlayAreaSelection(req.PlayArea))
	if err != nil {
		return countPlayAreaError(err), nil
	}

	count, err := h.uc.Count(ctx, game.PanoramaProvider(req.Provider), selection, actor)
	if err != nil {
		return countPlayAreaError(err), nil
	}

	return &api.PlayAreaCount{LocationCount: count}, nil
}

func (h Handler) CreatePlayArea(ctx context.Context, req *api.CreatePlayAreaRequest) (api.CreatePlayAreaRes, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return createPlayAreaError(err), nil
	}

	visibility, _ := req.Visibility.Get()

	saved, err := h.uc.Create(ctx, actor, req.Name, dto.PolygonFromAPI(req.Polygon), string(visibility))
	if err != nil {
		return createPlayAreaError(err), nil
	}

	return dto.SavedPlayAreaToAPI(saved), nil
}

func (h Handler) GetPlayArea(ctx context.Context, params api.GetPlayAreaParams) (api.GetPlayAreaRes, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return getPlayAreaError(err), nil
	}

	saved, err := h.uc.Get(ctx, params.ID, actor)
	if err != nil {
		return getPlayAreaError(err), nil
	}

	return dto.SavedPlayAreaToAPI(saved), nil
}

func (h Handler) DeletePlayArea(ctx context.Context, params api.DeletePlayAreaParams) (api.DeletePlayAreaRes, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return deletePlayAreaError(err), nil
	}

	if err := h.uc.Delete(ctx, params.ID, actor); err != nil {
		return deletePlayAreaError(err), nil
	}

	return &api.DeletePlayAreaNoContent{}, nil
}

func (h Handler) GetPlayAreas(ctx context.Context, params api.GetPlayAreasParams) (api.GetPlayAreasRes, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return getPlayAreasError(err), nil
	}

	saved, total, err := h.uc.List(ctx, actor, params.Page, params.PageSize)
	if err != nil {
		return getPlayAreasError(err), nil
	}

	result := &api.SavedPlayAreas{Total: total, Areas: make([]api.SavedPlayArea, 0, len(saved))}
	for _, value := range saved {
		result.Areas = append(result.Areas, *dto.SavedPlayAreaToAPI(value))
	}

	return result, nil
}

func (h Handler) UpdatePlayArea(
	ctx context.Context,
	req *api.UpdatePlayAreaRequest,
	params api.UpdatePlayAreaParams,
) (api.UpdatePlayAreaRes, error) {
	actor, err := h.actor(ctx)
	if err != nil {
		return updatePlayAreaError(err), nil
	}

	var (
		name, visibility *string
		polygon          *area.Polygon
	)

	if value, set := req.Name.Get(); set {
		name = &value
	}

	if value, set := req.Visibility.Get(); set {
		v := string(value)
		visibility = &v
	}

	if value, set := req.Polygon.Get(); set {
		p := dto.PolygonFromAPI(value)
		polygon = &p
	}

	saved, err := h.uc.Update(ctx, params.ID, actor, name, polygon, visibility)
	if err != nil {
		return updatePlayAreaError(err), nil
	}

	return dto.SavedPlayAreaToAPI(saved), nil
}

func (h Handler) GetPlayAreaPresets(
	ctx context.Context,
	params api.GetPlayAreaPresetsParams,
) (api.GetPlayAreaPresetsRes, error) {
	if _, err := h.actor(ctx); err != nil {
		return getPlayAreaPresetsError(err), nil
	}

	presets, err := h.uc.Presets(ctx, game.PanoramaProvider(params.Provider))
	if err != nil {
		return getPlayAreaPresetsError(err), nil
	}

	result := make(api.GetPlayAreaPresetsOKApplicationJSON, 0, len(presets))

	for _, value := range presets {
		p, err := dto.PlayAreaPresetToAPI(value, true)
		if err != nil {
			return getPlayAreaPresetsError(err), nil
		}

		result = append(result, *p)
	}

	return &result, nil
}

func (h Handler) GetPlayAreaPreset(
	ctx context.Context,
	params api.GetPlayAreaPresetParams,
) (api.GetPlayAreaPresetRes, error) {
	if _, err := h.actor(ctx); err != nil {
		return getPlayAreaPresetError(err), nil
	}

	preset, err := h.uc.Preset(ctx, params.ID)
	if err != nil {
		return getPlayAreaPresetError(err), nil
	}

	result, err := dto.PlayAreaPresetToAPI(preset, false)
	if err != nil {
		return getPlayAreaPresetError(err), nil
	}

	return result, nil
}

func countPlayAreaError(err error) api.CountPlayAreaRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.CountPlayAreaBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.CountPlayAreaUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.CountPlayAreaNotFound)(p)
	default:
		return (*api.CountPlayAreaInternalServerError)(p)
	}
}

func createPlayAreaError(err error) api.CreatePlayAreaRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.CreatePlayAreaBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.CreatePlayAreaUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.CreatePlayAreaNotFound)(p)
	default:
		return (*api.CreatePlayAreaInternalServerError)(p)
	}
}

func deletePlayAreaError(err error) api.DeletePlayAreaRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.DeletePlayAreaBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.DeletePlayAreaUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.DeletePlayAreaNotFound)(p)
	default:
		return (*api.DeletePlayAreaInternalServerError)(p)
	}
}

func getPlayAreaError(err error) api.GetPlayAreaRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.GetPlayAreaBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.GetPlayAreaUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.GetPlayAreaNotFound)(p)
	default:
		return (*api.GetPlayAreaInternalServerError)(p)
	}
}

func getPlayAreaPresetError(err error) api.GetPlayAreaPresetRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.GetPlayAreaPresetBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.GetPlayAreaPresetUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.GetPlayAreaPresetNotFound)(p)
	default:
		return (*api.GetPlayAreaPresetInternalServerError)(p)
	}
}

func getPlayAreaPresetsError(err error) api.GetPlayAreaPresetsRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.GetPlayAreaPresetsBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.GetPlayAreaPresetsUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.GetPlayAreaPresetsNotFound)(p)
	default:
		return (*api.GetPlayAreaPresetsInternalServerError)(p)
	}
}

func getPlayAreasError(err error) api.GetPlayAreasRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.GetPlayAreasBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.GetPlayAreasUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.GetPlayAreasNotFound)(p)
	default:
		return (*api.GetPlayAreasInternalServerError)(p)
	}
}

func updatePlayAreaError(err error) api.UpdatePlayAreaRes {
	p := problem(err)
	switch p.Status {
	case http.StatusBadRequest:
		return (*api.UpdatePlayAreaBadRequest)(p)
	case http.StatusUnauthorized:
		return (*api.UpdatePlayAreaUnauthorized)(p)
	case http.StatusNotFound:
		return (*api.UpdatePlayAreaNotFound)(p)
	default:
		return (*api.UpdatePlayAreaInternalServerError)(p)
	}
}

func (h Handler) actor(ctx context.Context) (uuid.UUID, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok || claims.UserID == uuid.Nil {
		return uuid.Nil, errUnauthorized
	}

	return claims.UserID, nil
}

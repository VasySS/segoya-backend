package user

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// GetPrivateProfile handles HTTP requests to retrieve a user's private profile.
func (h *Handler) GetPrivateProfile(ctx context.Context) (api.GetPrivateProfileRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetPrivateProfileUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	resp, err := h.uc.GetPrivateProfile(ctx, claims.UserID)
	if err != nil {
		slog.Error("error getting private profile", slog.Any("error", err))

		return &api.GetPrivateProfileInternalServerError{
			Title:  "Error getting profile",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting profile",
		}, nil
	}

	return dto.UserToAPIPrivateUser(resp), nil
}

// GetPublicProfile handles HTTP requests to retrieve a user's public profile.
func (h *Handler) GetPublicProfile(
	ctx context.Context,
	params api.GetPublicProfileParams,
) (api.GetPublicProfileRes, error) {
	resp, err := h.uc.GetPublicProfile(ctx, params.ID)
	if errors.Is(err, user.ErrUserNotFound) {
		return &api.GetPublicProfileNotFound{
			Title:  "User not found",
			Status: http.StatusNotFound,
			Detail: "The user you are trying to get does not exist",
		}, nil
	} else if err != nil {
		slog.Error("error getting public profile", slog.Any("error", err))

		return &api.GetPublicProfileInternalServerError{
			Title:  "Error getting profile",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting profile",
		}, nil
	}

	return dto.UserToAPIPublicUser(resp), nil
}

// UpdateUser handles HTTP requests to update user information.
func (h *Handler) UpdateUser(
	ctx context.Context,
	req *api.UserUpdateRequest,
) (api.UpdateUserRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.UpdateUserUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	err := h.uc.UpdateUser(ctx, dto.UpdateUserRequest{
		UserID: claims.UserID,
		Name:   req.GetName().Value,
	})
	if err != nil {
		slog.Error("error updating user", slog.Any("error", err))

		return &api.UpdateUserInternalServerError{
			Title:  "Error updating user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while updating user",
		}, nil
	}

	return &api.UpdateUserNoContent{}, nil
}

// UpdateUserAvatar handles HTTP requests to update a user's avatar.
func (h *Handler) UpdateUserAvatar(
	ctx context.Context,
	req *api.UpdateUserAvatarReq,
) (api.UpdateUserAvatarRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.UpdateUserAvatarUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	dtoReq := dto.UpdateAvatarRequest{
		RequestTime: time.Now().UTC(),
		UserID:      claims.UserID,
		File:        req.GetAvatarFile().File,
		MimeType:    req.GetAvatarFile().Header.Get("Content-Type"),
	}

	err := h.uc.UpdateAvatar(ctx, dtoReq)
	if errors.Is(err, user.ErrAvatarUpdateTooFrequent) {
		return &api.UpdateUserAvatarTooManyRequests{
			Title:  "Avatar update too frequent",
			Status: http.StatusTooManyRequests,
			Detail: "You can only update your avatar once every 5 minutes",
		}, nil
	} else if err != nil {
		slog.Error("error updating avatar", slog.Any("error", err))

		return &api.UpdateUserAvatarInternalServerError{
			Title:  "Error updating avatar",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while updating avatar",
		}, nil
	}

	return &api.UpdateUserAvatarNoContent{}, nil
}

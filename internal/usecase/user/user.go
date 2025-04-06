package user

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// GetPrivateProfile returns user's private profile.
func (uc Usecase) GetPrivateProfile(ctx context.Context, userID int) (user.PrivateProfile, error) {
	ctx, span := uc.tracer.Start(ctx, "GetPrivateProfile")
	defer span.End()

	userDB, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return user.PrivateProfile{}, fmt.Errorf("failed to get user profile: %w", err)
	}

	return userDB, nil
}

// GetPublicProfile returns user's public profile.
func (uc Usecase) GetPublicProfile(ctx context.Context, userID int) (user.PublicProfile, error) {
	ctx, span := uc.tracer.Start(ctx, "GetPublicProfile")
	defer span.End()

	userDB, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return user.PublicProfile{}, fmt.Errorf("failed to get user profile: %w", err)
	}

	return userDB.ToPublicProfile(), nil
}

// UpdateAvatar updates user's avatar.
func (uc Usecase) UpdateAvatar(ctx context.Context, req dto.UpdateAvatarRequest) error {
	ctx, span := uc.tracer.Start(ctx, "UpdateAvatar")
	defer span.End()

	err := uc.repo.RunTx(ctx, func(ctx context.Context) error {
		userRepo, err := uc.repo.GetUserByID(ctx, req.UserID)
		if err != nil {
			return fmt.Errorf("failed to get user from db: %w", err)
		}

		if userRepo.AvatarLastUpdate.Add(uc.cfg.AvatarUpdateLimit).After(req.RequestTime) {
			return user.ErrAvatarUpdateTooFrequent
		}

		// buffer is needed to read file body twice - for sha256 and later for S3 upload
		var fileBuf bytes.Buffer
		if _, err := io.Copy(&fileBuf, req.File); err != nil {
			return fmt.Errorf("failed to copy file into buffer: %w", err)
		}

		fileHash := sha256.Sum256(fileBuf.Bytes())
		fileHashStr := hex.EncodeToString(fileHash[:])

		if err := uc.s3.UploadAvatar(ctx, &fileBuf, fileHashStr, req.MimeType); err != nil {
			return fmt.Errorf("failed to upload avatar to s3: %w", err)
		}

		dbReq := dto.UpdateAvatarRequestDB{
			UserID:      req.UserID,
			AvatarHash:  fileHashStr,
			RequestTime: req.RequestTime,
		}

		if err := uc.repo.UpdateAvatar(ctx, dbReq); err != nil {
			return fmt.Errorf("failed to update avatar in db: %w", err)
		}

		if userRepo.AvatarHash != "" {
			if err := uc.s3.DeleteAvatar(ctx, userRepo.AvatarHash); err != nil {
				return fmt.Errorf("failed to delete old avatar from s3: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update avatar: %w", err)
	}

	return nil
}

// UpdateUser updates user info.
func (uc Usecase) UpdateUser(ctx context.Context, req dto.UpdateUserRequest) error {
	ctx, span := uc.tracer.Start(ctx, "UpdateUser")
	defer span.End()

	if err := uc.repo.UpdateUser(ctx, req); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update user info: %w", err)
	}

	return nil
}

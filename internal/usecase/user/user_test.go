package user_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	userEntity "github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/user"
	"github.com/VasySS/segoya-backend/internal/usecase/user/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUsecase_GetPrivateProfile(t *testing.T) {
	t.Parallel()

	fullUserProfile := userEntity.PrivateProfile{
		PublicProfile: userEntity.PublicProfile{
			ID:           1,
			Username:     "username",
			Name:         "name",
			RegisterDate: time.Now().UTC().Add(-24 * time.Hour),
			AvatarHash:   "",
		},
		Password:         "password_hash",
		AvatarLastUpdate: time.Time{},
	}

	type fields struct {
		repo *mocks.Repository
		s3   *mocks.S3Repository
	}

	type args struct {
		userID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    userEntity.PrivateProfile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get private profile",
			args: args{
				userID: fullUserProfile.ID,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetUserByID", mock.Anything, args.userID).Return(fullUserProfile, nil)
			},
			want:    fullUserProfile,
			wantErr: assert.NoError,
		},
		{
			name: "error when GetUserByID fails",
			args: args{
				userID: fullUserProfile.ID,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetUserByID", mock.Anything, args.userID).
					Return(userEntity.PrivateProfile{}, errors.New("database error"))
			},
			want: userEntity.PrivateProfile{},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "failed to get user profile: database error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			s3Repo := mocks.NewS3Repository(t)
			fs := fields{
				repo: repo,
				s3:   s3Repo,
			}
			uc := user.NewUsecase(
				user.Config{},
				repo,
				s3Repo,
			)

			tt.setup(fs, tt.args)

			got, err := uc.GetPrivateProfile(t.Context(), tt.args.userID)
			tt.wantErr(t, err, "Usecase.GetPrivateProfile() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equal(t, tt.want, got, "Usecase.GetPrivateProfile() = %v, want %v", got, tt.want)
		})
	}
}

func TestUsecase_GetPublicProfile(t *testing.T) {
	t.Parallel()

	fullUserProfile := userEntity.PrivateProfile{
		PublicProfile: userEntity.PublicProfile{
			ID:           1,
			Username:     "username",
			Name:         "name",
			RegisterDate: time.Now().UTC().Add(-24 * time.Hour),
			AvatarHash:   "",
		},
		Password:         "password_hash",
		AvatarLastUpdate: time.Time{},
	}

	type fields struct {
		repo *mocks.Repository
		s3   *mocks.S3Repository
	}

	type args struct {
		userID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    userEntity.PublicProfile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get public profile",
			args: args{
				userID: fullUserProfile.ID,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetUserByID", mock.Anything, args.userID).
					Return(fullUserProfile, nil)
			},
			want:    fullUserProfile.PublicProfile,
			wantErr: assert.NoError,
		},
		{
			name: "error when GetUserByID fails",
			args: args{
				userID: fullUserProfile.ID,
			},
			setup: func(fs fields, args args) {
				fs.repo.On("GetUserByID", mock.Anything, args.userID).
					Return(userEntity.PrivateProfile{}, errors.New("database error"))
			},
			want: userEntity.PublicProfile{},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "failed to get user profile: database error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			s3Repo := mocks.NewS3Repository(t)
			fs := fields{
				repo: repo,
				s3:   s3Repo,
			}
			uc := user.NewUsecase(
				user.Config{},
				repo,
				s3Repo,
			)

			tt.setup(fs, tt.args)

			got, err := uc.GetPublicProfile(t.Context(), tt.args.userID)
			tt.wantErr(t, err, "Usecase.GetPublicProfile() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equal(t, tt.want, got, "Usecase.GetPublicProfile() = %v, want %v", got, tt.want)
		})
	}
}

func TestUsecase_UpdateUser(t *testing.T) {
	t.Parallel()

	type fields struct {
		repo *mocks.Repository
		s3   *mocks.S3Repository
	}

	type args struct {
		req dto.UpdateUserRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully update user",
			args: args{
				req: dto.UpdateUserRequest{
					UserID: 1,
					Name:   "name",
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("UpdateUser", mock.Anything, args.req).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "error when UpdateUser fails",
			args: args{
				req: dto.UpdateUserRequest{
					UserID: 1,
					Name:   "name",
				},
			},
			setup: func(fs fields, args args) {
				fs.repo.On("UpdateUser", mock.Anything, args.req).
					Return(errors.New("database error"))
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "failed to update user info: database error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			s3Repo := mocks.NewS3Repository(t)
			fs := fields{
				repo: repo,
				s3:   s3Repo,
			}
			uc := user.NewUsecase(
				user.Config{},
				repo,
				s3Repo,
			)

			tt.setup(fs, tt.args)

			err := uc.UpdateUser(t.Context(), tt.args.req)
			tt.wantErr(t, err, "Usecase.UpdateUser() error = %v, wantErr %v", err, tt.wantErr)
		})
	}
}

type errorReader struct{}

func (er errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

//nolint:maintidx
func TestUsecase_UpdateUserAvatar(t *testing.T) {
	t.Parallel()

	avatarFileBytes := []byte("some avatar content")

	hash := sha256.Sum256(avatarFileBytes)
	expectedNewAvatarHash := hex.EncodeToString(hash[:])

	userPublicProfile := userEntity.PublicProfile{
		ID:         1,
		AvatarHash: "old_hash",
	}

	type fields struct {
		conf user.Config
		repo *mocks.Repository
		s3   *mocks.S3Repository
	}

	type args struct {
		req dto.UpdateAvatarRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successful update",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{
						PublicProfile:    userPublicProfile,
						AvatarLastUpdate: a.req.RequestTime.Add(-301 * time.Second),
					}, nil)

				f.repo.On("UpdateAvatar", mock.Anything, dto.UpdateAvatarRequestDB{
					UserID:      a.req.UserID,
					AvatarHash:  expectedNewAvatarHash,
					RequestTime: a.req.RequestTime,
				}).Return(nil)

				f.s3.On("UploadAvatar", mock.Anything,
					bytes.NewBuffer(avatarFileBytes), expectedNewAvatarHash, a.req.MimeType).
					Return(nil)

				f.s3.On("DeleteAvatar", mock.Anything, userPublicProfile.AvatarHash).
					Return(nil)
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        bytes.NewReader(avatarFileBytes),
					RequestTime: time.Now().UTC(),
					MimeType:    "image/webp",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "user not found",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{}, errors.New("user not found"))
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        bytes.NewReader(avatarFileBytes),
					RequestTime: time.Now().UTC(),
					MimeType:    "image/png",
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "user not found")
			},
		},
		{
			name: "avatar update too frequent",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{
						PublicProfile:    userPublicProfile,
						AvatarLastUpdate: a.req.RequestTime.Add(-299 * time.Second),
					}, nil)
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        bytes.NewReader(avatarFileBytes),
					RequestTime: time.Now().UTC(),
					MimeType:    "image/jpeg",
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "avatar update too frequent")
			},
		},
		{
			name: "error copying file",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{
						PublicProfile:    userPublicProfile,
						AvatarLastUpdate: a.req.RequestTime.Add(-48 * time.Hour),
					}, nil)
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        errorReader{},
					RequestTime: time.Now().UTC(),
					MimeType:    "image/avif",
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "read error")
			},
		},
		{
			name: "error uploading to S3",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{
						PublicProfile:    userPublicProfile,
						AvatarLastUpdate: a.req.RequestTime.Add(-301 * time.Second),
					}, nil)

				f.s3.On("UploadAvatar", mock.Anything,
					bytes.NewBuffer(avatarFileBytes), expectedNewAvatarHash, a.req.MimeType).
					Return(errors.New("s3 error"))
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        bytes.NewReader(avatarFileBytes),
					RequestTime: time.Now().UTC(),
					MimeType:    "image/gif",
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "s3 error")
			},
		},
		{
			name: "error updating avatar in DB",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.Anything).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{
						PublicProfile:    userPublicProfile,
						AvatarLastUpdate: a.req.RequestTime.Add(-1 * time.Hour),
					}, nil)

				f.repo.On("UpdateAvatar", mock.Anything, mock.Anything).
					Return(errors.New("db error"))

				f.s3.On("UploadAvatar", mock.Anything,
					bytes.NewBuffer(avatarFileBytes), expectedNewAvatarHash, a.req.MimeType).
					Return(nil)
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        bytes.NewReader(avatarFileBytes),
					RequestTime: time.Now().UTC(),
					MimeType:    "image/jpeg",
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "db error")
			},
		},
		{
			name: "error deleting old avatar",
			setup: func(f fields, a args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(func(ctx context.Context, fn repository.TxFunc) error {
						return fn(ctx)
					})

				f.repo.On("GetUserByID", mock.Anything, a.req.UserID).
					Return(userEntity.PrivateProfile{
						PublicProfile:    userPublicProfile,
						AvatarLastUpdate: a.req.RequestTime.Add(-24 * time.Hour),
					}, nil)

				f.repo.On("UpdateAvatar", mock.Anything, mock.Anything).
					Return(nil)

				f.s3.On("UploadAvatar", mock.Anything,
					bytes.NewBuffer(avatarFileBytes), expectedNewAvatarHash, a.req.MimeType).
					Return(nil)

				f.s3.On("DeleteAvatar", mock.Anything, userPublicProfile.AvatarHash).
					Return(errors.New("delete error"))
			},
			args: args{
				req: dto.UpdateAvatarRequest{
					UserID:      userPublicProfile.ID,
					File:        bytes.NewReader(avatarFileBytes),
					RequestTime: time.Now().UTC(),
					MimeType:    "image/jpeg",
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "delete error")
			},
		},
		{
			name: "RunTx error",
			setup: func(f fields, _ args) {
				f.repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
					Return(errors.New("tx error"))
			},
			args: args{
				req: dto.UpdateAvatarRequest{},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "tx error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewRepository(t)
			s3 := mocks.NewS3Repository(t)
			fs := fields{
				conf: user.Config{
					AvatarUpdateLimit: 5 * time.Minute,
				},
				repo: repo,
				s3:   s3,
			}
			tt.setup(fs, tt.args)

			uc := user.NewUsecase(
				fs.conf,
				fs.repo,
				fs.s3,
			)

			err := uc.UpdateAvatar(t.Context(), tt.args.req)
			tt.wantErr(t, err)
		})
	}
}

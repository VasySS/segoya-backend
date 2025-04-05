package cloudflare

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	// ErrNoFileName is returned when the file name is not provided.
	ErrNoFileName = errors.New("file name cannot be empty")
	// ErrNoFile is returned when the file is not provided.
	ErrNoFile = errors.New("file cannot be nil")
	// ErrNoMimeType is returned when the mime type is not provided.
	ErrNoMimeType = errors.New("mime type cannot be empty")
)

// UploadAvatar uploads avatar to Cloudflare.
func (r *Repository) UploadAvatar(ctx context.Context, file io.Reader, fileName, mimeType string) error {
	ctx, span := r.tracer.Start(ctx, "UploadAvatar")
	defer span.End()

	if file == nil {
		return ErrNoFile
	}

	if fileName == "" {
		return ErrNoFileName
	}

	if mimeType == "" {
		return ErrNoMimeType
	}

	command := &s3.PutObjectInput{
		Bucket:      aws.String(r.cfg.avatarBucketName),
		Key:         aws.String(fileName),
		Body:        file,
		ContentType: aws.String(mimeType),
	}

	if _, err := r.s3client.PutObject(ctx, command); err != nil {
		return fmt.Errorf("failed to upload avatar to cloudflare: %w", err)
	}

	return nil
}

// DeleteAvatar deletes avatar from Cloudflare.
func (r *Repository) DeleteAvatar(ctx context.Context, fileName string) error {
	ctx, span := r.tracer.Start(ctx, "DeleteAvatar")
	defer span.End()

	if fileName == "" {
		return ErrNoFileName
	}

	command := &s3.DeleteObjectInput{
		Bucket: &r.cfg.avatarBucketName,
		Key:    &fileName,
	}

	if _, err := r.s3client.DeleteObject(ctx, command); err != nil {
		return fmt.Errorf("failed to delete avatar from cloudflare: %w", err)
	}

	return nil
}

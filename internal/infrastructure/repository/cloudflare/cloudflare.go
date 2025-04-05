// Package cloudflare contains methods for working with Cloudflare repository.
package cloudflare

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Repository is a Cloudflare repository wrapper.
type Repository struct {
	cfg      Config
	s3client *s3.Client
	tracer   trace.Tracer
}

// New returns a new Cloudflare repository.
func New(
	ctx context.Context,
	cfg Config,
) (*Repository, error) {
	s3Cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"), // Cloudflare R2 expects "auto" as the region
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.accessKey,
				cfg.secretKey,
				"",
			),
		),
		config.WithRequestChecksumCalculation(0),
		config.WithResponseChecksumValidation(0),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load cloudflare config: %w", err)
	}

	s3client := s3.NewFromConfig(s3Cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.accountID))
	})

	return &Repository{
		s3client: s3client,
		tracer:   otel.GetTracerProvider().Tracer("CloudflareS3Repository"),
	}, nil
}

package cloudflare

import (
	"github.com/VasySS/segoya-backend/internal/config"
)

// Config contains configuration for Cloudflare R2 repository.
type Config struct {
	accessKey        string
	secretKey        string
	accountID        string
	avatarBucketName string
}

// NewConfig returns a new local configuration for Cloudflare R2 repository from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		accessKey:        conf.ENV.Cloudflare.BucketsAccessKey,
		secretKey:        conf.ENV.Cloudflare.BucketsSecretKey,
		accountID:        conf.ENV.Cloudflare.AccountID,
		avatarBucketName: conf.ENV.Cloudflare.AvatarBucket,
	}
}

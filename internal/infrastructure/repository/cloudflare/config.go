package cloudflare

import (
	"github.com/VasySS/segoya-backend/internal/config"
)

// Config is a Cloudflare repository configuration.
type Config struct {
	accessKey        string
	secretKey        string
	accountID        string
	avatarBucketName string
}

// NewConfig returns a new Cloudflare repository configuration from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		accessKey:        conf.ENV.CloudflareBucketsAccessKey,
		secretKey:        conf.ENV.CloudflareBucketsSecretKey,
		accountID:        conf.ENV.CloudflareAccountID,
		avatarBucketName: conf.ENV.CloudflareAvatarBucket,
	}
}

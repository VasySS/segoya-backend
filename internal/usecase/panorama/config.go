package panorama

import "github.com/VasySS/segoya-backend/internal/config"

// Config contains configuration for panorama usecase.
type Config struct{}

// NewConfig returns a new local config from general config.
func NewConfig(_ config.Config) Config {
	return Config{}
}

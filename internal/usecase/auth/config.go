package auth

import (
	"time"

	"golang.org/x/oauth2"

	"github.com/VasySS/segoya-backend/internal/config"
)

// Config contains configuration for auth usecase.
type Config struct {
	DiscordLogin    oauth2.Config
	DiscordNew      oauth2.Config
	YandexLogin     oauth2.Config
	YandexNew       oauth2.Config
	RefreshTokenTTL time.Duration
}

// NewConfig returns a new local auth config from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		DiscordLogin:    conf.OAuth.DiscordLogin,
		DiscordNew:      conf.OAuth.DiscordNew,
		YandexLogin:     conf.OAuth.YandexLogin,
		YandexNew:       conf.OAuth.YandexNew,
		RefreshTokenTTL: conf.Limits.RefreshTokenTTL,
	}
}

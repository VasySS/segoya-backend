package auth

import (
	"net/http"
	"time"

	"github.com/VasySS/segoya-backend/internal/config"
	"golang.org/x/oauth2"
)

// Config contains configuration for auth usecase.
type Config struct {
	HTTPClientProxy *http.Client
	DiscordLogin    oauth2.Config
	DiscordNew      oauth2.Config
	YandexLogin     oauth2.Config
	YandexNew       oauth2.Config
	YandexSecretKey string
	RefreshTokenTTL time.Duration
}

// NewConfig returns a new local config from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		HTTPClientProxy: conf.HTTPClient,
		DiscordLogin:    conf.OAuth.DiscordLogin,
		DiscordNew:      conf.OAuth.DiscordNew,
		YandexLogin:     conf.OAuth.YandexLogin,
		YandexNew:       conf.OAuth.YandexNew,
		YandexSecretKey: conf.ENV.YandexSecretKey,
		RefreshTokenTTL: conf.Limits.RefreshTokenTTL,
	}
}

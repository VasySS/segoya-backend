package auth

import (
	"net/url"
	"time"

	"github.com/VasySS/segoya-backend/internal/config"
	"golang.org/x/oauth2"
)

type oauthConfig struct {
	oauthCookieTTL time.Duration
	oauthStateLen  int
	yandexLogin    oauth2.Config
	yandexNew      oauth2.Config
	discordLogin   oauth2.Config
	discordNew     oauth2.Config
}

// Config contains configuration for auth HTTP handlers.
type Config struct {
	oauthConfig
	captchaSecretKey string
	frontendURL      url.URL
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
}

// NewConfig creates and returns new local config from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		oauthConfig: oauthConfig{
			oauthCookieTTL: conf.OAuth.CookieTTL,
			oauthStateLen:  conf.OAuth.StateLen,
			yandexLogin:    conf.OAuth.YandexLogin,
			yandexNew:      conf.OAuth.YandexNew,
			discordLogin:   conf.OAuth.DiscordLogin,
			discordNew:     conf.OAuth.DiscordNew,
		},
		captchaSecretKey: conf.ENV.CaptchaSecretKey,
		frontendURL:      conf.ENV.FrontendURL,
		accessTokenTTL:   conf.Limits.AccessTokenTTL,
		refreshTokenTTL:  conf.Limits.RefreshTokenTTL,
	}
}

package config

import (
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// OAuth contains settings for OAuth providers.
type OAuth struct {
	StateLen     int
	CookieTTL    time.Duration
	DiscordLogin oauth2.Config
	DiscordNew   oauth2.Config
	YandexLogin  oauth2.Config
	YandexNew    oauth2.Config
}

const (
	yandexHandlerPath  = "/v1/auth/yandex"
	discordHandlerPath = "/v1/auth/discord"
)

func newOAuthConfig(backendURL url.URL, conf Config) OAuth {
	createConfig := func(base oauth2.Config, redirectPath string) oauth2.Config {
		return oauth2.Config{
			ClientID:     base.ClientID,
			ClientSecret: base.ClientSecret,
			RedirectURL:  redirectPath,
			Scopes:       base.Scopes,
			Endpoint:     base.Endpoint,
		}
	}

	discordBase := oauth2.Config{
		ClientID:     conf.ENV.DiscordAccountID,
		ClientSecret: conf.ENV.DiscordSecretKey,
		Scopes:       []string{"openid"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/api/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}

	yandexBase := oauth2.Config{
		ClientID:     conf.ENV.YandexAccountID,
		ClientSecret: conf.ENV.YandexSecretKey,
		Scopes:       []string{"login:email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://oauth.yandex.ru/authorize",
			TokenURL: "https://oauth.yandex.ru/token",
		},
	}

	return OAuth{
		CookieTTL:    10 * time.Minute,
		StateLen:     32,
		DiscordLogin: createConfig(discordBase, backendURL.String()+discordHandlerPath+"/login/callback"),
		DiscordNew:   createConfig(discordBase, backendURL.String()+discordHandlerPath+"/new/callback"),
		YandexLogin:  createConfig(yandexBase, backendURL.String()+yandexHandlerPath+"/login/callback"),
		YandexNew:    createConfig(yandexBase, backendURL.String()+yandexHandlerPath+"/new/callback"),
	}
}

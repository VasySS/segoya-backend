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

func newOAuthConfig(conf Config) OAuth {
	newRedirectURL := func(backendURL url.URL, path string) string {
		return backendURL.ResolveReference(&url.URL{Path: path}).String()
	}

	createConfig := func(base oauth2.Config, redirectPath string) oauth2.Config {
		return oauth2.Config{
			ClientID:     base.ClientID,
			ClientSecret: base.ClientSecret,
			RedirectURL:  newRedirectURL(conf.ENV.BackendURL, redirectPath),
			Scopes:       base.Scopes,
			Endpoint:     base.Endpoint,
		}
	}

	discordBase := oauth2.Config{
		ClientID:     conf.ENV.DiscordOAuth.ClientID,
		ClientSecret: conf.ENV.DiscordOAuth.ClientSecret,
		Scopes:       []string{"openid"},
		//nolint:gosec
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/api/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}

	yandexBase := oauth2.Config{
		ClientID:     conf.ENV.YandexOAuth.ClientID,
		ClientSecret: conf.ENV.YandexOAuth.ClientSecret,
		Scopes:       []string{"login:email"},
		//nolint:gosec
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://oauth.yandex.ru/authorize",
			TokenURL: "https://oauth.yandex.ru/token",
		},
	}

	return OAuth{
		CookieTTL:    10 * time.Minute,
		StateLen:     32,
		DiscordLogin: createConfig(discordBase, discordHandlerPath+"/login/callback"),
		DiscordNew:   createConfig(discordBase, discordHandlerPath+"/new/callback"),
		YandexLogin:  createConfig(yandexBase, yandexHandlerPath+"/login/callback"),
		YandexNew:    createConfig(yandexBase, yandexHandlerPath+"/new/callback"),
	}
}

// Package config provides configuration for the application.
package config

import (
	"log"
	"log/slog"
	"net/http"
	"net/url"

	httpPkg "github.com/VasySS/segoya-backend/pkg/http"
	"github.com/ilyakaznacheev/cleanenv"
)

// Proxy contains proxy settings.
type Proxy struct {
	Address  string `env:"PROXY_ADDR"`
	Username string `env:"PROXY_USERNAME"`
	Password string `env:"PROXY_PASSWORD"`
}

// Cloudflare contains cloudflare settings.
type Cloudflare struct {
	CloudflareAvatarBucket     string `env:"CLOUDFLARE_AVATAR_BUCKET"`
	CloudflareBucketsAccessKey string `env:"CLOUDFLARE_BUCKETS_ACCESS_KEY"`
	CloudflareBucketsSecretKey string `env:"CLOUDFLARE_BUCKETS_SECRET_KEY"`
	CloudflareAccountID        string `env:"CLOUDFLARE_ACCOUNT_ID"`
}

// DiscordOAuth contains discord oauth settings.
type DiscordOAuth struct {
	DiscordAccountID string `env:"DISCORD_ACCOUNT_ID"`
	DiscordSecretKey string `env:"DISCORD_SECRET_KEY"`
}

// YandexOAuth contains yandex oauth settings.
type YandexOAuth struct {
	YandexAccountID string `env:"YANDEX_ACCOUNT_ID"`
	YandexSecretKey string `env:"YANDEX_SECRET_KEY"`
}

// Postgres contains Postgres connection credentials.
type Postgres struct {
	PostgresUser     string `env:"PG_USER" env-required:"true"`
	PostgresPassword string `env:"PG_PASS" env-required:"true"`
	PostgresHost     string `env:"PG_HOST" env-required:"true"`
	PostgresDatabase string `env:"PG_DB"   env-required:"true"`
}

// Config contains application configuration.
type Config struct {
	ENV struct {
		Postgres
		Proxy
		Cloudflare
		DiscordOAuth
		YandexOAuth
		BackendURL       url.URL `env:"BACKEND_URL"        env-required:"true"`
		FrontendURL      url.URL `env:"FRONTEND_URL"       env-required:"true"`
		ValkeyURL        string  `env:"VALKEY_URL"         env-required:"true"`
		JaegerURL        string  `env:"JAEGER_URL"         env-required:"true"`
		CaptchaSecretKey string  `env:"CAPTCHA_SECRET_KEY" env-required:"true"`
		JWTSecretKey     string  `env:"JWT_SECRET_KEY"     env-required:"true"`
		Mode             string  `env:"ENV_MODE"           env-default:"production"`
	}
	HTTPClient *http.Client
	OAuth      OAuth
	Limits     Limits
}

// MustInit reads environment variables and returns a new global config.
func MustInit() Config {
	var conf Config

	if err := cleanenv.ReadConfig(".env", &conf.ENV); err != nil {
		slog.Info("failed to read .env, using environment variables")
	}

	if err := cleanenv.ReadEnv(&conf.ENV); err != nil {
		log.Fatal(err)
	}

	conf.OAuth = newOAuthConfig(conf.ENV.BackendURL, conf)
	conf.Limits = newLimits()

	proxyClient, err := httpPkg.NewClientWithProxy(
		conf.ENV.Address,
		conf.ENV.Username,
		conf.ENV.Password,
	)
	if err != nil {
		slog.Info("failed to create proxy client, using default client", slog.Any("error", err))

		conf.HTTPClient = httpPkg.NewClient()
	} else {
		conf.HTTPClient = proxyClient
	}

	return conf
}

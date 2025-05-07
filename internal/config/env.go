package config

import "net/url"

// Proxy contains SOCKS5 proxy settings.
type Proxy struct {
	Address  string `env:"PROXY_ADDR"`
	Username string `env:"PROXY_USERNAME"`
	Password string `env:"PROXY_PASSWORD"`
}

// Cloudflare contains cloudflare settings.
type Cloudflare struct {
	AvatarBucket     string `env:"CLOUDFLARE_AVATAR_BUCKET"`
	BucketsAccessKey string `env:"CLOUDFLARE_BUCKETS_ACCESS_KEY"`
	BucketsSecretKey string `env:"CLOUDFLARE_BUCKETS_SECRET_KEY"`
	AccountID        string `env:"CLOUDFLARE_ACCOUNT_ID"`
}

// DiscordOAuth contains discord oauth settings.
type DiscordOAuth struct {
	ClientID     string `env:"DISCORD_CLIENT_ID"`
	ClientSecret string `env:"DISCORD_CLIENT_SECRET"`
}

// YandexOAuth contains yandex oauth settings.
type YandexOAuth struct {
	ClientID     string `env:"YANDEX_CLIENT_ID"`
	ClientSecret string `env:"YANDEX_CLIENT_SECRET"`
}

// Optional contains all optional environment settings.
type Optional struct {
	Proxy            Proxy
	Cloudflare       Cloudflare
	DiscordOAuth     DiscordOAuth
	YandexOAuth      YandexOAuth
	CaptchaSecretKey string `env:"CAPTCHA_SECRET_KEY"`
	Mode             string `env:"ENV_MODE"           env-default:"production"`
}

// Postgres contains Postgres connection credentials.
type Postgres struct {
	User     string `env:"PG_USER" env-required:"true"`
	Password string `env:"PG_PASS" env-required:"true"`
	Host     string `env:"PG_HOST" env-required:"true"`
	Database string `env:"PG_DB"   env-required:"true"`
}

// Required contains all required environment settings.
type Required struct {
	Postgres     Postgres
	BackendURL   url.URL `env:"BACKEND_URL"    env-required:"true"`
	FrontendURL  url.URL `env:"FRONTEND_URL"   env-required:"true"`
	ValkeyURL    string  `env:"VALKEY_URL"     env-required:"true"`
	JaegerURL    string  `env:"JAEGER_URL"     env-required:"true"`
	JWTSecretKey string  `env:"JWT_SECRET_KEY" env-required:"true"`
}

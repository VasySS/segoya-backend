package user

import "time"

// OAuthIssuer is a type of OAuth provider.
type OAuthIssuer string

// Available OAuth providers.
const (
	DiscordOAuth OAuthIssuer = "discord"
	YandexOAuth  OAuthIssuer = "yandex"
)

// OAuth contains user's connected OAuth information.
type OAuth struct {
	ID        string
	UserID    int
	OAuthID   string `db:"oauth_id"`
	Issuer    OAuthIssuer
	CreatedAt time.Time
}

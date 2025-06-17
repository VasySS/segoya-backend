package user

import (
	"time"

	"github.com/google/uuid"
)

// OAuthIssuer is a type of OAuth provider.
type OAuthIssuer string

// Available OAuth providers.
const (
	DiscordOAuth OAuthIssuer = "discord"
	YandexOAuth  OAuthIssuer = "yandex"
)

// OAuth contains user's connected OAuth information.
type OAuth struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	OAuthID   string `db:"oauth_id"`
	Issuer    OAuthIssuer
	CreatedAt time.Time
}

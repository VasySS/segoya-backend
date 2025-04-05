package lobby

import (
	"time"

	"github.com/VasySS/segoya-backend/internal/config"
)

// Config contains lobby configuration.
type Config struct {
	LobbyExpiration time.Duration
	LobbyIDLength   int
}

// NewConfig returns a new local lobby config from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		LobbyExpiration: conf.Limits.LobbyExpiration,
		LobbyIDLength:   conf.Limits.LobbyIDLength,
	}
}

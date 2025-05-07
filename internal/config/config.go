// Package config provides configuration for the application.
package config

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/ilyakaznacheev/cleanenv"

	httpPkg "github.com/VasySS/segoya-backend/pkg/http"
)

// Config contains application configuration.
type Config struct {
	ENV struct {
		Optional
		Required
	}
	// HTTPClient is used for making external requests, using proxy if provided.
	HTTPClient *http.Client
	OAuth      OAuth
	Limits     Limits
}

// MustInit reads environment variables and returns a new global config.
func MustInit() Config {
	var conf Config

	if err := cleanenv.ReadConfig(".env", &conf.ENV); err != nil {
		slog.Info("failed to read .env file, trying to use environment variables")
	}

	if err := cleanenv.ReadEnv(&conf.ENV); err != nil {
		slog.Error("failed to read environment variables", slog.Any("error", err))
		os.Exit(1)
	}

	conf.OAuth = newOAuthConfig(conf)
	conf.Limits = newLimits()

	if conf.ENV.Proxy.Address != "" {
		proxyClient, err := httpPkg.NewClientWithSOCKS5(
			conf.ENV.Proxy.Address,
			conf.ENV.Proxy.Username,
			conf.ENV.Proxy.Password,
		)
		if err != nil {
			slog.Error("failed to create proxy client", slog.Any("error", err))
			os.Exit(1)
		}

		conf.HTTPClient = proxyClient
	} else {
		conf.HTTPClient = httpPkg.NewClient()
	}

	return conf
}

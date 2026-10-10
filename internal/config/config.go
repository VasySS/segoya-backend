// Package config provides configuration for the application.
package config

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"

	httpPkg "github.com/VasySS/segoya-backend/pkg/http"
)

var (
	// ErrUnsupportedCaptchaProvider indicates an unsupported CAPTCHA_PROVIDER setting.
	ErrUnsupportedCaptchaProvider = errors.New("CAPTCHA_PROVIDER must be turnstile or yandex")
	// ErrCaptchaSecretRequired indicates a missing secret outside development mode.
	ErrCaptchaSecretRequired = errors.New("CAPTCHA_SECRET_KEY is required outside development mode")
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

	if err := conf.ValidateCaptcha(); err != nil {
		slog.Error("invalid captcha configuration", slog.Any("error", err))
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

// ValidateCaptcha applies the default provider and validates CAPTCHA settings before startup.
func (c *Config) ValidateCaptcha() error {
	if c.ENV.CaptchaProvider == "" {
		c.ENV.CaptchaProvider = "turnstile"
	}

	switch c.ENV.CaptchaProvider {
	case "turnstile", "yandex":
	default:
		return ErrUnsupportedCaptchaProvider
	}

	if strings.TrimSpace(c.ENV.CaptchaSecretKey) == "" && c.ENV.Mode != "development" {
		return ErrCaptchaSecretRequired
	}

	return nil
}

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/VasySS/segoya-backend/internal/config"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/auth"
	"github.com/VasySS/segoya-backend/pkg/captcha"
)

//nolint:ireturn // Provider selection uses the existing authentication verification interface.
func newCaptchaService(conf config.Config) (auth.CaptchaService, error) {
	if err := conf.ValidateCaptcha(); err != nil {
		return nil, fmt.Errorf("validate captcha settings: %w", err)
	}

	if conf.ENV.Mode == "development" && strings.TrimSpace(conf.ENV.CaptchaSecretKey) == "" {
		return developmentCaptchaService{}, nil
	}

	switch conf.ENV.CaptchaProvider {
	case "turnstile":
		return captcha.NewCloudflareService(conf.HTTPClient, conf.ENV.CaptchaSecretKey), nil
	case "yandex":
		return captcha.NewYandexService(conf.HTTPClient, conf.ENV.CaptchaSecretKey), nil
	case "cap":
		return captcha.NewCapService(conf.HTTPClient, conf.ENV.CaptchaVerifyURL, conf.ENV.CaptchaSecretKey), nil
	default:
		return nil, config.ErrUnsupportedCaptchaProvider
	}
}

type developmentCaptchaService struct{}

func (developmentCaptchaService) IsTokenValid(_ context.Context, _ string) error {
	return nil
}

package auth_test

import (
	"testing"

	"golang.org/x/oauth2"

	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/auth"
)

func TestUsecase_YandexOAuth(t *testing.T) {
	t.Parallel()
	testOAuthProvider(t, oauthProvider{
		issuer: user.YandexOAuth, exchange: "ExchangeYandexCodeForID",
		linkConfig:  func(c auth.Config) oauth2.Config { return c.YandexNew },
		loginConfig: func(c auth.Config) oauth2.Config { return c.YandexLogin },
		begin:       (*auth.Usecase).NewYandex, link: (*auth.Usecase).NewYandexCallback,
		login: (*auth.Usecase).LoginYandexCallback, remove: (*auth.Usecase).DeleteYandex,
	})
}

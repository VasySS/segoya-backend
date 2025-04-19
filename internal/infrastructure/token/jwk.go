package token

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// https://stackoverflow.com/questions/61850992/jwt-validation-with-jwks-golang
//
//nolint:ireturn
func newCachedJWKSet(ctx context.Context, jwkURL string, httpClient *http.Client) jwk.Set {
	jwkCache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		slog.Error("error creating jwk cache", slog.Any("error", err))
		os.Exit(1)
	}

	// register a minimum refresh interval for URL.
	// when not specified, defaults to Cache-Control and similar resp headers
	err = jwkCache.Register(ctx, jwkURL,
		jwk.WithMinInterval(10*time.Minute),
		jwk.WithHTTPClient(httpClient),
	)
	if err != nil {
		slog.Error("error registering jwk", slog.Any("error", err))
		os.Exit(1)
	}

	// fetch once on application startup to check that url is valid
	_, err = jwkCache.Refresh(ctx, jwkURL)
	if err != nil {
		slog.Error("error refreshing jwk", slog.Any("error", err))
		os.Exit(1)
	}

	cachedSet, err := jwkCache.CachedSet(jwkURL)
	if err != nil {
		slog.Error("error getting jwk set", slog.Any("error", err))
		os.Exit(1)
	}

	return cachedSet
}

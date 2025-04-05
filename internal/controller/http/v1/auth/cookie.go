package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
)

// ErrCookieParsing is returned when an error occurs while parsing a cookie.
var ErrCookieParsing = errors.New("error parsing cookie")

const (
	accessCookieName  = "accessToken"
	refreshCookieName = "refreshToken"
	stateCookieName   = "oauthState"
)

// newCookieStringFromTokens creates a cookie string from the access and refresh tokens
// for using in the Set-Cookie header.
func (h Handler) newCookieStringFromTokens(access, refresh string) string {
	frontendURL := h.cfg.frontendURL.Hostname()

	// Path is set because of this:
	// https://svelte.dev/docs/kit/migrating-to-sveltekit-2#path-is-required-when-setting-cookies
	accessCookie := http.Cookie{
		Name:     accessCookieName,
		Value:    access,
		Path:     "/",
		Domain:   frontendURL,
		MaxAge:   int(h.cfg.accessTokenTTL.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	refreshCookie := http.Cookie{
		Name:     refreshCookieName,
		Value:    refresh,
		Path:     "/",
		Domain:   frontendURL,
		MaxAge:   int(h.cfg.refreshTokenTTL.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	return accessCookie.String() + "," + refreshCookie.String()
}

// newOAuthCookieState creates a cookie string from the oauth state for using in the Set-Cookie header.
func (h Handler) newOAuthCookieState(state string) string {
	cookieTTL := h.cfg.oauthCookieTTL
	frontendURL := h.cfg.frontendURL.Hostname()

	stateCookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		Domain:   frontendURL,
		MaxAge:   int(cookieTTL.Seconds()),
		Secure:   true,
		HttpOnly: true,
	}

	return stateCookie.String()
}

func (h Handler) parseCookieState(cookie string) (string, error) {
	cookies, err := http.ParseCookie(cookie)
	if err != nil {
		return "", fmt.Errorf("error parsing cookie: %w", err)
	}

	cookieIdx := slices.IndexFunc(cookies, func(c *http.Cookie) bool {
		return c.Name == stateCookieName
	})

	if cookieIdx == -1 {
		return "", ErrCookieParsing
	}

	stateCookieValue, err := url.QueryUnescape(cookies[cookieIdx].Value)
	if err != nil || stateCookieValue == "" {
		return "", fmt.Errorf("error parsing cookie: %w", err)
	}

	return stateCookieValue, nil
}

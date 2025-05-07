// Package token contains methods for working with JWT tokens.
package token

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// SignatureMethod is a default signature method used to sign tokens.
//
//nolint:gochecknoglobals
var SignatureMethod = jwa.HS256()

const (
	parsingAcceptableSkew = 3 * time.Minute
	discordOAuthKeysURL   = "https://discord.com/api/oauth2/keys"
)

type tokenCtxKey struct{}

// Service is a service for working with tokens.
type Service struct {
	jwtSecret       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	httpClient      *http.Client
	yandexSecretKey string
	jwkSet          jwk.Set
}

// NewService creates new token service.
func NewService(
	ctx context.Context,
	jwtSecret string,
	accessTokenTTL, refreshTokenTTL time.Duration,
	yandexSecretKey string,
	httpClient *http.Client,
) *Service {
	jwkSet := newCachedJWKSet(ctx, discordOAuthKeysURL, httpClient)

	return &Service{
		jwtSecret:       []byte(jwtSecret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		httpClient:      httpClient,
		yandexSecretKey: yandexSecretKey,
		jwkSet:          jwkSet,
	}
}

// NewAccessToken creates new access token string.
func (s *Service) NewAccessToken(current time.Time, req user.AccessTokenClaims) (string, error) {
	expirationTime := current.Add(s.accessTokenTTL)

	accessToken, err := jwt.NewBuilder().
		IssuedAt(current).
		Expiration(expirationTime).
		Claim(ClaimsSessionIDKey, req.SessionID).
		Claim(ClaimsUserIDKey, req.UserID).
		Claim(ClaimsUsernameKey, req.Username).
		Claim(ClaimsNameKey, req.Name).
		Claim(ClaimsTokenTypeKey, AccessToken).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build access token: %w", err)
	}

	signedToken, err := jwt.Sign(accessToken, jwt.WithKey(SignatureMethod, s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return string(signedToken), nil
}

// NewRefreshToken creates new refresh token string.
func (s *Service) NewRefreshToken(current time.Time, req user.RefreshTokenClaims) (string, error) {
	expirationTime := current.Add(s.refreshTokenTTL)

	refreshToken, err := jwt.NewBuilder().
		IssuedAt(current).
		Expiration(expirationTime).
		Claim(ClaimsSessionIDKey, req.SessionID).
		Claim(ClaimsUserIDKey, req.UserID).
		Claim(ClaimsUsernameKey, req.Username).
		Claim(ClaimsTokenTypeKey, RefreshToken).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build refresh token: %w", err)
	}

	signedToken, err := jwt.Sign(refreshToken, jwt.WithKey(SignatureMethod, s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return string(signedToken), nil
}

// ParseAccessToken parses access token and returns its claims.
func (s *Service) ParseAccessToken(token string) (user.AccessTokenClaims, error) {
	accessToken, err := jwt.ParseString(token,
		jwt.WithKey(SignatureMethod, s.jwtSecret),
		jwt.WithAcceptableSkew(parsingAcceptableSkew),
	)
	if err != nil {
		return user.AccessTokenClaims{}, fmt.Errorf("error parsing access token: %w", err)
	}

	tokenType, err := GetType(accessToken)
	if err != nil {
		return user.AccessTokenClaims{}, err
	}

	if tokenType != AccessToken {
		return user.AccessTokenClaims{}, user.ErrWrongTokenType
	}

	sessionID, err := GetSessionID(accessToken)
	if err != nil {
		return user.AccessTokenClaims{}, err
	}

	userID, err := GetUserID(accessToken)
	if err != nil {
		return user.AccessTokenClaims{}, err
	}

	username, err := GetUsername(accessToken)
	if err != nil {
		return user.AccessTokenClaims{}, err
	}

	name, err := GetName(accessToken)
	if err != nil {
		return user.AccessTokenClaims{}, err
	}

	return user.AccessTokenClaims{
		SessionID: sessionID,
		UserID:    userID,
		Username:  username,
		Name:      name,
	}, nil
}

// ParseRefreshToken parses refresh token and returns its claims.
func (s *Service) ParseRefreshToken(token string) (user.RefreshTokenClaims, error) {
	refreshToken, err := jwt.ParseString(token,
		jwt.WithKey(SignatureMethod, s.jwtSecret),
		jwt.WithAcceptableSkew(parsingAcceptableSkew),
	)
	if err != nil {
		return user.RefreshTokenClaims{}, fmt.Errorf("error parsing refresh token: %w", err)
	}

	tokenType, err := GetType(refreshToken)
	if err != nil {
		return user.RefreshTokenClaims{}, err
	}

	if tokenType != RefreshToken {
		return user.RefreshTokenClaims{}, user.ErrWrongTokenType
	}

	sessionID, err := GetSessionID(refreshToken)
	if err != nil {
		return user.RefreshTokenClaims{}, err
	}

	userID, err := GetUserID(refreshToken)
	if err != nil {
		return user.RefreshTokenClaims{}, err
	}

	username, err := GetUsername(refreshToken)
	if err != nil {
		return user.RefreshTokenClaims{}, err
	}

	return user.RefreshTokenClaims{
		SessionID: sessionID,
		UserID:    userID,
		Username:  username,
	}, nil
}

// NewContext adds the access token claims to the context and returns it.
func (s *Service) NewContext(ctx context.Context, token user.AccessTokenClaims) context.Context {
	ctx = context.WithValue(ctx, tokenCtxKey{}, token)

	return ctx
}

// FromContext returns the access token claims from the context.
func (s *Service) FromContext(ctx context.Context) (user.AccessTokenClaims, bool) {
	token, ok := ctx.Value(tokenCtxKey{}).(user.AccessTokenClaims)
	return token, ok
}

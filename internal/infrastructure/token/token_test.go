package token_test

import (
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/token"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupService(jwtSecret []byte, accessTokenTTL, refreshTokenTTL time.Duration) *token.Service {
	return token.NewService(string(jwtSecret), accessTokenTTL, refreshTokenTTL)
}

func TestNewAccessToken(t *testing.T) {
	t.Parallel()

	var (
		jwtSecret       = []byte("jwt-secret")
		accessTokenTTL  = time.Hour
		refreshTokenTTL = 31 * 24 * time.Hour
	)

	service := setupService(jwtSecret, accessTokenTTL, refreshTokenTTL)
	currentTime := time.Now().UTC()

	claims := user.AccessTokenClaims{
		SessionID: "session-123",
		UserID:    456,
		Username:  "testuser",
		Name:      "Test User",
	}

	t.Run("Valid Access Token", func(t *testing.T) {
		t.Parallel()

		tokenStr, err := service.NewAccessToken(currentTime, claims)
		require.NoError(t, err)
		assert.NotEmpty(t, tokenStr)

		parsedClaims, err := service.ParseAccessToken(tokenStr)
		require.NoError(t, err)
		assert.Equal(t, claims, parsedClaims)
	})
}

func TestNewRefreshToken(t *testing.T) {
	t.Parallel()

	var (
		jwtSecret       = []byte("jwt-secret")
		accessTokenTTL  = time.Hour
		refreshTokenTTL = 31 * 24 * time.Hour
	)

	service := setupService(jwtSecret, accessTokenTTL, refreshTokenTTL)
	currentTime := time.Now()

	claims := user.RefreshTokenClaims{
		SessionID: "session-123",
		UserID:    456,
		Username:  "testuser",
	}

	t.Run("Valid Refresh Token", func(t *testing.T) {
		t.Parallel()

		tokenStr, err := service.NewRefreshToken(currentTime, claims)
		require.NoError(t, err)
		assert.NotEmpty(t, tokenStr)

		parsedClaims, err := service.ParseRefreshToken(tokenStr)
		require.NoError(t, err)
		assert.Equal(t, claims, parsedClaims)
	})
}

func TestParseAccessToken(t *testing.T) {
	t.Parallel()

	var (
		jwtSecret       = []byte("jwt-secret")
		accessTokenTTL  = time.Hour
		refreshTokenTTL = 31 * 24 * time.Hour
	)

	service := setupService(jwtSecret, accessTokenTTL, refreshTokenTTL)
	now := time.Now().UTC()

	claims := user.AccessTokenClaims{
		SessionID: "session-123",
		UserID:    456,
		Username:  "testuser",
		Name:      "Test User",
	}

	t.Run("Valid Access Token", func(t *testing.T) {
		t.Parallel()

		tokenStr, err := service.NewAccessToken(now, claims)
		require.NoError(t, err)

		parsedClaims, err := service.ParseAccessToken(tokenStr)
		require.NoError(t, err)
		assert.Equal(t, claims, parsedClaims)
	})

	t.Run("Expired Access Token", func(t *testing.T) {
		t.Parallel()

		builder := jwt.NewBuilder().
			IssuedAt(now.Add(-2*time.Hour)).
			Expiration(now.Add(-1*time.Hour)).
			Claim(token.ClaimsSessionIDKey, claims.SessionID).
			Claim(token.ClaimsUserIDKey, claims.UserID).
			Claim(token.ClaimsNameKey, claims.Name).
			Claim(token.ClaimsUsernameKey, claims.Username).
			Claim(token.ClaimsTokenTypeKey, token.AccessToken)

		rawToken, err := builder.Build()
		require.NoError(t, err)

		signedToken, err := jwt.Sign(rawToken, jwt.WithKey(token.SignatureMethod, jwtSecret))
		require.NoError(t, err)

		_, err = service.ParseAccessToken(string(signedToken))
		assert.ErrorContains(t, err, "\"exp\" not satisfied")
	})

	t.Run("Wrong token type", func(t *testing.T) {
		t.Parallel()

		builder := jwt.NewBuilder().
			IssuedAt(now).
			Expiration(now.Add(refreshTokenTTL)).
			Claim(token.ClaimsSessionIDKey, claims.SessionID).
			Claim(token.ClaimsUserIDKey, claims.UserID).
			Claim(token.ClaimsNameKey, claims.Name).
			Claim(token.ClaimsUsernameKey, claims.Username).
			Claim(token.ClaimsTokenTypeKey, token.RefreshToken)

		rawToken, err := builder.Build()
		require.NoError(t, err)

		signedToken, err := jwt.Sign(rawToken, jwt.WithKey(token.SignatureMethod, jwtSecret))
		require.NoError(t, err)

		_, err = service.ParseAccessToken(string(signedToken))
		assert.ErrorIs(t, err, user.ErrWrongTokenType)
	})

	t.Run("Missing Required Claim", func(t *testing.T) {
		t.Parallel()

		// Build token without 'name' claim
		builder := jwt.NewBuilder().
			IssuedAt(now).
			Expiration(now.Add(accessTokenTTL)).
			Claim(token.ClaimsSessionIDKey, claims.SessionID).
			Claim(token.ClaimsUserIDKey, claims.UserID).
			Claim(token.ClaimsUsernameKey, claims.Username).
			Claim(token.ClaimsTokenTypeKey, token.AccessToken)

		rawToken, err := builder.Build()
		require.NoError(t, err)

		signedToken, err := jwt.Sign(rawToken, jwt.WithKey(token.SignatureMethod, jwtSecret))
		require.NoError(t, err)

		_, err = service.ParseAccessToken(string(signedToken))
		assert.Error(t, err)
	})
}

func TestParseRefreshToken(t *testing.T) {
	t.Parallel()

	var (
		jwtSecret       = []byte("jwt-secret")
		accessTokenTTL  = time.Hour
		refreshTokenTTL = 31 * 24 * time.Hour
	)

	service := setupService(jwtSecret, accessTokenTTL, refreshTokenTTL)
	now := time.Now().UTC()

	claims := user.RefreshTokenClaims{
		SessionID: "session-123",
		UserID:    456,
		Username:  "testuser",
	}

	t.Run("Valid Refresh Token", func(t *testing.T) {
		t.Parallel()

		tokenStr, err := service.NewRefreshToken(now, claims)
		require.NoError(t, err)

		parsedClaims, err := service.ParseRefreshToken(tokenStr)
		require.NoError(t, err)
		assert.Equal(t, claims, parsedClaims)
	})

	t.Run("Expired Refresh Token", func(t *testing.T) {
		t.Parallel()

		builder := jwt.NewBuilder().
			IssuedAt(now.Add(-96*time.Hour)).
			Expiration(now.Add(-48*time.Hour)).
			Claim(token.ClaimsSessionIDKey, claims.SessionID).
			Claim(token.ClaimsUserIDKey, claims.UserID).
			Claim(token.ClaimsUsernameKey, claims.Username).
			Claim(token.ClaimsTokenTypeKey, token.RefreshToken)

		rawToken, err := builder.Build()
		require.NoError(t, err)

		signedToken, err := jwt.Sign(rawToken, jwt.WithKey(token.SignatureMethod, jwtSecret))
		require.NoError(t, err)

		_, err = service.ParseRefreshToken(string(signedToken))
		assert.ErrorContains(t, err, "\"exp\" not satisfied")
	})

	t.Run("Wrong Token Type", func(t *testing.T) {
		t.Parallel()

		// Build refresh token with access token type
		builder := jwt.NewBuilder().
			IssuedAt(now).
			Expiration(now.Add(refreshTokenTTL)).
			Claim(token.ClaimsSessionIDKey, claims.SessionID).
			Claim(token.ClaimsUserIDKey, claims.UserID).
			Claim(token.ClaimsUsernameKey, claims.Username).
			Claim(token.ClaimsTokenTypeKey, token.AccessToken)

		rawToken, err := builder.Build()
		require.NoError(t, err)

		signedToken, err := jwt.Sign(rawToken, jwt.WithKey(token.SignatureMethod, jwtSecret))
		require.NoError(t, err)

		_, err = service.ParseRefreshToken(string(signedToken))
		assert.ErrorIs(t, err, user.ErrWrongTokenType)
	})

	t.Run("Missing Required Claim", func(t *testing.T) {
		t.Parallel()

		// Build token without UserID
		builder := jwt.NewBuilder().
			IssuedAt(now).
			Expiration(now.Add(refreshTokenTTL)).
			Claim(token.ClaimsSessionIDKey, claims.SessionID).
			Claim(token.ClaimsUsernameKey, claims.Username).
			Claim(token.ClaimsTokenTypeKey, token.RefreshToken)

		rawToken, err := builder.Build()
		require.NoError(t, err)

		signedToken, err := jwt.Sign(rawToken, jwt.WithKey(token.SignatureMethod, jwtSecret))
		require.NoError(t, err)

		_, err = service.ParseRefreshToken(string(signedToken))
		assert.Error(t, err)
	})
}

func TestContext(t *testing.T) {
	t.Parallel()

	var (
		jwtSecret       = []byte("jwt-secret")
		accessTokenTTL  = time.Hour
		refreshTokenTTL = 31 * 24 * time.Hour
	)

	service := setupService(jwtSecret, accessTokenTTL, refreshTokenTTL)

	claims := user.AccessTokenClaims{
		SessionID: "session-123",
		UserID:    456,
		Username:  "testuser",
		Name:      "Test User",
	}

	t.Run("Valid Context", func(t *testing.T) {
		t.Parallel()

		newCtx := service.NewContext(t.Context(), claims)
		retrievedClaims, ok := service.FromContext(newCtx)
		assert.True(t, ok)
		assert.Equal(t, claims, retrievedClaims)
	})

	t.Run("Missing Context", func(t *testing.T) {
		t.Parallel()

		retrievedClaims, ok := service.FromContext(t.Context())
		assert.False(t, ok)
		assert.Empty(t, retrievedClaims)
	})
}

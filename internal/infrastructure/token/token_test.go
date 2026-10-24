package token_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/token"
)

func newTokenService(t *testing.T) *token.Service {
	t.Helper()
	return token.NewService(t.Context(), "jwt-secret", time.Hour, 31*24*time.Hour, "", &http.Client{})
}

func tokenClaims() user.AccessTokenClaims {
	return user.AccessTokenClaims{
		SessionID: uuid.MustParse("019a553a-fc00-7000-8000-000000000002"),
		UserID:    uuid.MustParse("019a553a-fc00-7000-8000-000000000001"),
		Username:  "alice", Name: "Alice",
	}
}

func TestService_TokenRoundTrip(t *testing.T) {
	t.Parallel()

	for _, kind := range []token.Type{token.AccessToken, token.RefreshToken} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			service := newTokenService(t)
			// Parsing uses the real clock; keep validity far from the three-minute skew boundary.
			now := time.Now().UTC().Truncate(time.Second)
			claims := tokenClaims()

			var (
				encoded string
				err     error
			)

			ttl := time.Hour

			if kind == token.AccessToken {
				encoded, err = service.NewAccessToken(now, claims)
			} else {
				ttl = 31 * 24 * time.Hour
				encoded, err = service.NewRefreshToken(now, user.RefreshTokenClaims{
					SessionID: claims.SessionID, UserID: claims.UserID, Username: claims.Username,
				})
			}

			require.NoError(t, err)
			require.NotEmpty(t, encoded)
			parsed, err := jwt.ParseString(encoded, jwt.WithKey(jwa.HS256(), []byte("jwt-secret")))
			require.NoError(t, err)

			issued, ok := parsed.IssuedAt()
			require.True(t, ok)
			assert.Equal(t, now, issued)

			expires, ok := parsed.Expiration()
			require.True(t, ok)
			assert.Equal(t, now.Add(ttl), expires)

			if kind == token.AccessToken {
				got, err := service.ParseAccessToken(encoded)
				require.NoError(t, err)
				assert.Equal(t, claims, got)
			} else {
				got, err := service.ParseRefreshToken(encoded)
				require.NoError(t, err)
				assert.Equal(t, user.RefreshTokenClaims{
					SessionID: claims.SessionID,
					UserID:    claims.UserID,
					Username:  claims.Username,
				}, got)
			}
		})
	}
}

func signedClaims(
	t *testing.T, kind token.Type, change func(map[string]any), secret string, algorithm jwa.SignatureAlgorithm,
) string {
	t.Helper()

	claims := tokenClaims()
	now := time.Now().UTC()

	values := map[string]any{
		"iat": now.Add(-time.Minute), "exp": now.Add(time.Hour),
		token.ClaimsTokenTypeKey: string(kind), token.ClaimsSessionIDKey: claims.SessionID.String(),
		token.ClaimsUserIDKey: claims.UserID.String(), token.ClaimsUsernameKey: claims.Username,
	}
	if kind == token.AccessToken {
		values[token.ClaimsNameKey] = claims.Name
	}

	if change != nil {
		change(values)
	}

	raw := jwt.New()
	for key, value := range values {
		require.NoError(t, raw.Set(key, value))
	}

	encoded, err := jwt.Sign(raw, jwt.WithKey(algorithm, []byte(secret)))
	require.NoError(t, err)

	return string(encoded)
}

func assertRejectedToken(t *testing.T, kind token.Type, encoded string, want error) {
	t.Helper()

	service := newTokenService(t)
	if kind == token.AccessToken {
		claims, err := service.ParseAccessToken(encoded)
		require.Error(t, err)

		if want != nil {
			require.ErrorIs(t, err, want)
		}

		assert.Equal(t, user.AccessTokenClaims{}, claims)
	} else {
		claims, err := service.ParseRefreshToken(encoded)
		require.Error(t, err)

		if want != nil {
			require.ErrorIs(t, err, want)
		}

		assert.Equal(t, user.RefreshTokenClaims{}, claims)
	}
}

func TestService_RejectInvalidTokenEnvelope(t *testing.T) {
	t.Parallel()

	for _, kind := range []token.Type{token.AccessToken, token.RefreshToken} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			for _, scenario := range []string{
				"empty",
				"malformed",
				"unsigned",
				"wrong secret",
				"wrong algorithm",
				"tampered payload",
				"expired",
				"issued in future",
				"wrong type",
			} {
				t.Run(scenario, func(t *testing.T) {
					t.Parallel()

					encoded, want := invalidTokenEnvelope(t, kind, scenario)
					assertRejectedToken(t, kind, encoded, want)
				})
			}
		})
	}
}

func TestService_RejectInvalidRequiredClaims(t *testing.T) {
	t.Parallel()

	for _, kind := range []token.Type{token.AccessToken, token.RefreshToken} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			keys := []string{token.ClaimsTokenTypeKey, token.ClaimsSessionIDKey, token.ClaimsUserIDKey, token.ClaimsUsernameKey}
			if kind == token.AccessToken {
				keys = append(keys, token.ClaimsNameKey)
			}

			for _, key := range keys {
				for _, corruption := range []string{"missing", "wrong JSON type", "null"} {
					t.Run(key+"/"+corruption, func(t *testing.T) {
						t.Parallel()
						encoded := signedClaims(t, kind, func(values map[string]any) {
							switch corruption {
							case "missing":
								delete(values, key)
							case "wrong JSON type":
								values[key] = 42
							case "null":
								values[key] = nil
							}
						}, "jwt-secret", jwa.HS256())
						assertRejectedToken(t, kind, encoded, nil)
					})
				}
			}

			for _, key := range []string{token.ClaimsSessionIDKey, token.ClaimsUserIDKey} {
				t.Run(key+"/invalid UUID", func(t *testing.T) {
					t.Parallel()
					encoded := signedClaims(t, kind, func(values map[string]any) {
						values[key] = "not-a-uuid"
					}, "jwt-secret", jwa.HS256())
					assertRejectedToken(t, kind, encoded, nil)
				})
			}
		})
	}
}

func TestService_Context(t *testing.T) {
	t.Parallel()
	service := newTokenService(t)
	claims := tokenClaims()
	ctx := service.NewContext(t.Context(), claims)
	got, ok := service.FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, claims, got)
	missing, ok := service.FromContext(t.Context())
	assert.False(t, ok)
	assert.Equal(t, user.AccessTokenClaims{}, missing)
}

func invalidTokenEnvelope(t *testing.T, kind token.Type, scenario string) (string, error) {
	t.Helper()

	var (
		encoded string
		want    error
	)

	switch scenario {
	case "empty":
		encoded = ""
	case "malformed":
		encoded = "not.a.jwt"
	case "unsigned":
		encoded = "eyJhbGciOiJub25lIn0.e30."
	case "wrong secret":
		encoded = signedClaims(t, kind, nil, "other-secret", jwa.HS256())
	case "wrong algorithm":
		encoded = signedClaims(t, kind, nil, "jwt-secret", jwa.HS512())
	case "tampered payload":
		parts := strings.Split(signedClaims(t, kind, nil, "jwt-secret", jwa.HS256()), ".")
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)

		var values map[string]any

		require.NoError(t, json.Unmarshal(payload, &values))
		values[token.ClaimsUsernameKey] = "mallory"
		payload, err = json.Marshal(values)
		require.NoError(t, err)

		parts[1] = base64.RawURLEncoding.EncodeToString(payload)
		encoded = strings.Join(parts, ".")
	case "expired":
		encoded = signedClaims(t, kind, func(v map[string]any) {
			v["iat"] = time.Now().Add(-2 * time.Hour)
			v["exp"] = time.Now().Add(-time.Hour)
		}, "jwt-secret", jwa.HS256())
	case "issued in future":
		encoded = signedClaims(t, kind, func(v map[string]any) {
			v["iat"] = time.Now().Add(time.Hour)
			v["exp"] = time.Now().Add(2 * time.Hour)
		}, "jwt-secret", jwa.HS256())
	case "wrong type":
		other := map[token.Type]token.Type{
			token.AccessToken: token.RefreshToken, token.RefreshToken: token.AccessToken,
		}[kind]

		encoded = signedClaims(t, other, nil, "jwt-secret", jwa.HS256())
		want = user.ErrWrongTokenType
	}

	return encoded, want
}

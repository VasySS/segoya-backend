package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// TokenService is a service for working with tokens.
type TokenService interface {
	NewContext(ctx context.Context, token user.AccessTokenClaims) context.Context
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
	ParseAccessToken(token string) (user.AccessTokenClaims, error)
}

// Auth is a middleware for authorizing http requests.
type Auth struct {
	tokenService TokenService
}

// NewAuth creates a new auth middleware that checks JWT tokens.
func NewAuth(tokenService TokenService) Auth {
	return Auth{
		tokenService: tokenService,
	}
}

// HandleBearer is a middleware for authorizing http requests (where token is sent as a header).
func (mw Auth) HandleBearer(
	ctx context.Context,
	_ api.OperationName,
	t api.Bearer,
) (context.Context, error) {
	tokenClaims, err := mw.tokenService.ParseAccessToken(t.GetToken())
	if err != nil {
		return ctx, fmt.Errorf("invalid token: %w", err)
	}

	return mw.tokenService.NewContext(ctx, tokenClaims), nil
}

// HandleWS is a middleware for authorizing websockets (where token is sent as a query parameter).
func (mw Auth) HandleWS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		queryToken := r.URL.Query().Get("token")
		if queryToken == "" {
			http.Error(w, "no token in query", http.StatusUnauthorized)
			return
		}

		token, err := mw.tokenService.ParseAccessToken(queryToken)
		if err != nil {
			slog.Debug(err.Error())
			http.Error(w, "invalid token", http.StatusUnauthorized)

			return
		}

		ctx = mw.tokenService.NewContext(ctx, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

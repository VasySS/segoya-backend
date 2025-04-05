package middleware

import (
	"net/http"
)

// CORS is a middleware for setting CORS headers.
func CORS(frontendURL string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", frontendURL)
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Origin, Content-Type, Accept, Authorization, X-Request-With, Set-Cookie, Cookie, Bearer, "+
					"X-Captcha-Token",
			)
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

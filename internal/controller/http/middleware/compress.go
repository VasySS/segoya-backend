package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Compress is a middleware for compressing responses (skipping websockets).
func Compress(next http.Handler) http.Handler {
	compressor := middleware.Compress(5, "text/plain", "application/json")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		compressor(next).ServeHTTP(w, r)
	})
}

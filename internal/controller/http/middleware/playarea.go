package middleware

import (
	"bytes"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/VasySS/segoya-backend/internal/entity/playarea"
)

// PlayAreaBodyLimit bounds memory before generated JSON decoders process polygons.
func PlayAreaBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := path.Clean(r.URL.Path)

		limited := requestPath == "/v1/singleplayer" || requestPath == "/v1/lobbies" ||
			requestPath == "/v1/play-areas" || strings.HasPrefix(requestPath, "/v1/play-areas/")
		if !limited || (r.Method != http.MethodPost && r.Method != http.MethodPatch) {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, playarea.MaxBodyBytes+1))
		_ = r.Body.Close()

		if err != nil {
			http.Error(w, "unable to read request body", http.StatusBadRequest)
			return
		}

		if len(body) > playarea.MaxBodyBytes {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = io.WriteString(w, `{
				"title": "Request Entity Too Large",
				"status": 413,
				"detail": "Play area requests must not exceed 64 KiB"
			}`)

			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

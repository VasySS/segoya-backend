package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Logger is a middleware for logging requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/openapi") {
			next.ServeHTTP(w, r)
			return
		}

		remoteAddr := strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]
		if remoteAddr == "" {
			remoteAddr = r.RemoteAddr
		}

		entry := slog.With(
			slog.String("request_id", middleware.GetReqID(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", remoteAddr),
			// slog.String("user_agent", r.UserAgent()),
		)

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		t1 := time.Now().UTC()

		defer func() {
			status := ww.Status()
			duration := time.Since(t1)

			switch {
			case status >= 500:
				entry.Error("Server error",
					slog.Int("status", status),
					slog.String("duration", duration.String()))
			case status >= 400:
				entry.Warn("Request not successful",
					slog.Int("status", status),
					slog.String("duration", duration.String()))
			default:
				entry.Info("Request was successful",
					slog.Int("status", status),
					slog.String("duration", duration.String()),
				)
			}
		}()

		next.ServeHTTP(ww, r)
	})
}

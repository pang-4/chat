package server

import (
	"log/slog"
	"net/http"
	"time"
)

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)

		if r.URL.Path != "/ws" {
			logger.Info(
				"http request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(started),
			)
		}
	})
}

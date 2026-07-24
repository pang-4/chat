package server

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/pang-4/chat/internal/chat"
	"github.com/pang-4/chat/internal/database"
)

type readinessChecker interface {
	Ping(ctx context.Context) error
}

func newRouter(db *sql.DB, redis readinessChecker, hub *chat.Hub, websocketHandler http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/ws", websocketHandler)
	mux.HandleFunc("GET /health/live", liveHandler)
	mux.HandleFunc("GET /health/ready", readyHandler(db, redis))
	mux.HandleFunc("GET /stats", statsHandler(hub))

	return mux
}

func liveHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func readyHandler(db *sql.DB, redis readinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if err := database.CheckSchema(ctx, db); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if err := redis.Ping(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func statsHandler(hub *chat.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connections":` + strconv.FormatInt(hub.ConnectionCount(), 10) + `}`))
	}
}

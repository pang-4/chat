package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pang-4/chat/internal/chat"
	"github.com/pang-4/chat/internal/config"
	"github.com/pang-4/chat/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Open(appCtx, cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.Migrate(appCtx, db); err != nil {
		return err
	}

	hub := chat.NewHub(logger)
	go hub.Run(appCtx)

	repository := database.NewMessageRepository(db)
	websocketHandler := chat.NewHandler(hub, repository, logger, cfg.Server.AllowedOrigins)

	mux := http.NewServeMux()
	mux.Handle("/ws", websocketHandler)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connections":` + strconv.FormatInt(hub.ConnectionCount(), 10) + `}`))
	})

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           requestLogger(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server started", "address", cfg.Server.Address)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case sig := <-signals:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("server shutdown complete")
	return nil
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/ws" {
			logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
		}
	})
}

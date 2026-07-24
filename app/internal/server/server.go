package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/pang-4/chat/internal/chat"
	"github.com/pang-4/chat/internal/config"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func New(
	cfg config.Server,
	db *sql.DB,
	redis readinessChecker,
	hub *chat.Hub,
	websocketHandler http.Handler,
	logger *slog.Logger,
) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.Address,
			Handler:           requestLogger(logger, newRouter(db, redis, hub, websocketHandler)),
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		logger: logger,
	}
}

func (s *Server) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)
	go func() {
		s.logger.Info("server started", "address", s.httpServer.Addr)
		serverErrors <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.logger.Info("server shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}

	s.logger.Info("server shutdown complete")
	return nil
}

package application

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pang-4/chat/internal/chat"
	"github.com/pang-4/chat/internal/config"
	"github.com/pang-4/chat/internal/database"
	httpserver "github.com/pang-4/chat/internal/server"
)

func Run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := database.CheckSchema(ctx, db); err != nil {
		return err
	}

	hub := chat.NewHub(logger)
	go hub.Run(ctx)

	repository := database.NewMessageRepository(db)
	websocketHandler := chat.NewHandler(hub, repository, logger, cfg.Server.AllowedOrigins)
	server := httpserver.New(cfg.Server, db, hub, websocketHandler, logger)

	return server.Run(ctx)
}

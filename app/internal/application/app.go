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
	"github.com/pang-4/chat/internal/messaging"
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

	messageBroker, err := messaging.NewRedis(ctx, cfg.Redis, logger)
	if err != nil {
		return err
	}
	defer messageBroker.Close()
	if err := messageBroker.Subscribe(ctx, func(payload []byte) {
		if !hub.Broadcast(payload) {
			logger.Warn("local broadcast skipped because hub stopped")
		}
	}); err != nil {
		return err
	}

	repository := database.NewMessageRepository(db)
	websocketHandler := chat.NewHandler(hub, repository, messageBroker, logger, cfg.Server.AllowedOrigins)
	server := httpserver.New(cfg.Server, db, messageBroker, hub, websocketHandler, logger)

	return server.Run(ctx)
}

package main

import (
	"log/slog"
	"os"

	"github.com/pang-4/chat/internal/application"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := application.Run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

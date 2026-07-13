package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/pang-4/chat/internal/config"
	"github.com/pang-4/chat/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: go run ./cmd/migrate [up|down|version]")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	runner, err := migrate.New(cfg.Migrations.SourceURL, database.ConnectionURL(cfg.Database))
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := runner.Close()
		if sourceErr != nil {
			logger.Warn("migration source close failed", "error", sourceErr)
		}
		if databaseErr != nil {
			logger.Warn("migration database close failed", "error", databaseErr)
		}
	}()

	switch args[0] {
	case "up":
		err = runner.Up()
	case "down":
		// Roll back only the most recent migration to avoid accidental full data removal.
		err = runner.Steps(-1)
	case "version":
		version, dirty, versionErr := runner.Version()
		if versionErr != nil {
			if errors.Is(versionErr, migrate.ErrNilVersion) {
				logger.Info("migration version", "version", 0, "dirty", false)
				return nil
			}
			return versionErr
		}
		logger.Info("migration version", "version", version, "dirty", dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q: use up, down, or version", args[0])
	}

	if errors.Is(err, migrate.ErrNoChange) {
		logger.Info("no migration changes")
		return nil
	}
	if err != nil {
		return err
	}

	logger.Info("migration complete", "command", args[0])
	return nil
}

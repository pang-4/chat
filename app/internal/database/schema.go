package database

import (
	"context"
	"database/sql"
	"fmt"
)

func CheckSchema(ctx context.Context, db *sql.DB) error {
	var messagesTableExists bool
	if err := db.QueryRowContext(
		ctx,
		`SELECT to_regclass('public.messages') IS NOT NULL`,
	).Scan(&messagesTableExists); err != nil {
		return fmt.Errorf("check database schema: %w", err)
	}

	if !messagesTableExists {
		return fmt.Errorf("database schema is not migrated: run go run ./cmd/migrate up")
	}

	return nil
}

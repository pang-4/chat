package database

import (
	"context"
	"database/sql"
	"fmt"
)

func Migrate(ctx context.Context, db *sql.DB) error {

	query := `
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		sender TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("create messages table: %w", err)
	}

	return nil
}

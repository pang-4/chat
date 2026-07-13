package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pang-4/chat/internal/chat"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Save(ctx context.Context, message *chat.Message) error {
	const query = `
		INSERT INTO messages (sender, content)
		VALUES ($1, $2)
		RETURNING id, created_at
	`

	if err := r.db.QueryRowContext(ctx, query, message.Sender, message.Content).
		Scan(&message.ID, &message.CreatedAt); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

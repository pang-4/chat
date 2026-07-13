package chat

import "context"

type MessageRepository interface {
	Save(ctx context.Context, message *Message) error
}

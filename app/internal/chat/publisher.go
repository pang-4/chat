package chat

import "context"

// MessagePublisher sends a message to every chat server instance.
type MessagePublisher interface {
	Publish(ctx context.Context, message Message) error
}

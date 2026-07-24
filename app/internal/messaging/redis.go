package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/pang-4/chat/internal/chat"
	"github.com/pang-4/chat/internal/config"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client  *redis.Client
	channel string
	logger  *slog.Logger
}

func NewRedis(ctx context.Context, cfg config.Redis, logger *slog.Logger) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.Database,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to redis: %w", err)
	}

	return &Redis{
		client:  client,
		channel: cfg.Channel,
		logger:  logger,
	}, nil
}

func (r *Redis) Publish(ctx context.Context, message chat.Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode redis message: %w", err)
	}
	if err := r.client.Publish(ctx, r.channel, payload).Err(); err != nil {
		return fmt.Errorf("publish redis message: %w", err)
	}
	return nil
}

func (r *Redis) Subscribe(ctx context.Context, handle func([]byte)) error {
	pubsub := r.client.Subscribe(ctx, r.channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return fmt.Errorf("subscribe redis channel: %w", err)
	}

	go func() {
		defer pubsub.Close()
		channel := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-channel:
				if !ok {
					r.logger.Error("redis subscription closed", "channel", r.channel)
					return
				}
				handle([]byte(message.Payload))
			}
		}
	}()

	r.logger.Info("redis subscription started", "channel", r.channel)
	return nil
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.client.Close()
}

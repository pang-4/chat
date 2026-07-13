package chat

import (
	"context"
	"log/slog"
	"sync/atomic"
)

const defaultClientQueueSize = 256

type delivery struct {
	client  *Client
	payload []byte
}

type Hub struct {
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	direct     chan delivery
	done       chan struct{}
	count      atomic.Int64
	logger     *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan []byte, 256),
		direct:     make(chan delivery, 64),
		done:       make(chan struct{}),
		logger:     logger,
	}
}

func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)

	for {
		select {
		case <-ctx.Done():
			for client := range h.clients {
				close(client.send)
				client.closeConnection()
				delete(h.clients, client)
			}
			h.count.Store(0)
			return

		case client := <-h.register:
			h.clients[client] = struct{}{}
			h.count.Add(1)
			h.logger.Info("websocket connected", "connections", h.count.Load(), "remote_addr", client.remoteAddress())

		case client := <-h.unregister:
			h.remove(client, "disconnected")

		case payload := <-h.broadcast:
			for client := range h.clients {
				if !h.enqueue(client, payload) {
					h.remove(client, "send queue full")
				}
			}

		case item := <-h.direct:
			if _, exists := h.clients[item.client]; exists && !h.enqueue(item.client, item.payload) {
				h.remove(item.client, "send queue full")
			}
		}
	}
}

func (h *Hub) Register(client *Client) bool {
	select {
	case h.register <- client:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

func (h *Hub) Broadcast(payload []byte) bool {
	select {
	case h.broadcast <- payload:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) Send(client *Client, payload []byte) bool {
	select {
	case h.direct <- delivery{client: client, payload: payload}:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) ConnectionCount() int64 {
	return h.count.Load()
}

func (h *Hub) enqueue(client *Client, payload []byte) bool {
	select {
	case client.send <- payload:
		return true
	default:
		return false
	}
}

func (h *Hub) remove(client *Client, reason string) {
	if _, exists := h.clients[client]; !exists {
		return
	}

	delete(h.clients, client)
	h.count.Add(-1)
	close(client.send)
	client.closeConnection()
	h.logger.Info("websocket disconnected", "connections", h.count.Load(), "remote_addr", client.remoteAddress(), "reason", reason)
}

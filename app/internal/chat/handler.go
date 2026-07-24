package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const (
	maxNameLength    = 50
	maxContentLength = 2000
)

type Handler struct {
	hub              *Hub
	repository       MessageRepository
	publisher        MessagePublisher
	logger           *slog.Logger
	allowEveryOrigin bool
	allowedOrigins   map[string]struct{}
	upgrader         websocket.Upgrader
}

func NewHandler(
	hub *Hub,
	repository MessageRepository,
	publisher MessagePublisher,
	logger *slog.Logger,
	allowedOrigins []string,
) *Handler {
	handler := &Handler{
		hub:            hub,
		repository:     repository,
		publisher:      publisher,
		logger:         logger,
		allowedOrigins: make(map[string]struct{}, len(allowedOrigins)),
	}
	for _, origin := range allowedOrigins {
		if origin == "*" {
			handler.allowEveryOrigin = true
			continue
		}
		handler.allowedOrigins[strings.TrimRight(origin, "/")] = struct{}{}
	}
	handler.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     handler.checkOrigin,
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("websocket upgrade failed", "error", err, "remote_addr", r.RemoteAddr)
		return
	}

	client := NewClient(conn)
	go client.writePump()
	if !h.hub.Register(client) {
		client.closeConnection()
		return
	}

	defer func() {
		h.hub.Unregister(client)
		if client.Name() != "" {
			h.publish(r.Context(), NewSystemMessage(client.Name()+"님이 퇴장했습니다."))
		}
	}()

	client.readPump(func(payload []byte) {
		h.handleMessage(r, client, payload)
	})
}

func (h *Handler) handleMessage(r *http.Request, client *Client, payload []byte) {
	var incoming IncomingMessage
	if err := json.Unmarshal(payload, &incoming); err != nil {
		h.sendError(client, "올바른 JSON 메시지가 아닙니다.")
		return
	}

	switch incoming.Type {
	case MessageTypeJoin:
		h.handleJoin(r.Context(), client, incoming)
	case MessageTypeChat:
		h.handleChat(r, client, incoming)
	default:
		h.sendError(client, "지원하지 않는 메시지 타입입니다.")
	}
}

func (h *Handler) handleJoin(ctx context.Context, client *Client, incoming IncomingMessage) {
	if client.Name() != "" {
		h.sendError(client, "이미 입장한 사용자입니다.")
		return
	}

	name := strings.TrimSpace(incoming.Sender)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		h.sendError(client, "이름은 1자 이상 50자 이하여야 합니다.")
		return
	}

	client.setName(name)
	if err := h.publish(ctx, NewSystemMessage(name+"님이 입장했습니다.")); err != nil {
		h.sendError(client, "입장 알림을 전달하지 못했습니다.")
	}
}

func (h *Handler) handleChat(r *http.Request, client *Client, incoming IncomingMessage) {
	if client.Name() == "" {
		h.sendError(client, "채팅 전에 먼저 입장해야 합니다.")
		return
	}

	content := strings.TrimSpace(incoming.Content)
	if content == "" || utf8.RuneCountInString(content) > maxContentLength {
		h.sendError(client, "메시지는 1자 이상 2000자 이하여야 합니다.")
		return
	}

	message := Message{
		Type:    MessageTypeChat,
		Sender:  client.Name(),
		Content: content,
	}
	if err := h.repository.Save(r.Context(), &message); err != nil {
		h.logger.Error("message save failed", "error", err, "sender", client.Name())
		h.sendError(client, "메시지를 저장하지 못했습니다. 잠시 후 다시 시도해주세요.")
		return
	}

	if err := h.publish(r.Context(), message); err != nil {
		h.sendError(client, "메시지를 전달하지 못했습니다. 잠시 후 다시 시도해주세요.")
	}
}

func (h *Handler) publish(ctx context.Context, message Message) error {
	if err := h.publisher.Publish(ctx, message); err != nil {
		h.logger.Error("message publish failed", "error", err, "type", message.Type, "sender", message.Sender)
		return err
	}
	return nil
}

func (h *Handler) sendError(client *Client, content string) {
	payload, err := json.Marshal(NewErrorMessage(content))
	if err != nil {
		h.logger.Error("error message encoding failed", "error", err)
		return
	}
	h.hub.Send(client, payload)
}

func (h *Handler) checkOrigin(r *http.Request) bool {
	if h.allowEveryOrigin {
		return true
	}
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	if origin == "" {
		return true
	}
	if _, err := url.ParseRequestURI(origin); err != nil {
		return false
	}
	_, allowed := h.allowedOrigins[origin]
	return allowed
}

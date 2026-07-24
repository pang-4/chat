package chat

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeMessageRepository struct {
	mu       sync.Mutex
	messages []Message
}

type localPublisher struct {
	hub *Hub
}

func (p *localPublisher) Publish(_ context.Context, message Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	p.hub.Broadcast(payload)
	return nil
}

func (r *fakeMessageRepository) Save(_ context.Context, message *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	message.ID = int64(len(r.messages) + 1)
	message.CreatedAt = time.Date(2026, time.July, 13, 0, 0, 0, 0, time.UTC)
	r.messages = append(r.messages, *message)
	return nil
}

func TestWebSocketChatFlow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	repository := &fakeMessageRepository{}
	handler := NewHandler(hub, repository, &localPublisher{hub: hub}, logger, []string{"*"})
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	first := dialWebSocket(t, wsURL)
	defer first.Close()

	writeJSON(t, first, IncomingMessage{Type: MessageTypeJoin, Sender: "alice"})
	assertMessage(t, first, MessageTypeSystem, "alice님이 입장했습니다.")

	second := dialWebSocket(t, wsURL)
	defer second.Close()
	writeJSON(t, second, IncomingMessage{Type: MessageTypeJoin, Sender: "bob"})
	assertMessage(t, first, MessageTypeSystem, "bob님이 입장했습니다.")
	assertMessage(t, second, MessageTypeSystem, "bob님이 입장했습니다.")

	writeJSON(t, first, IncomingMessage{Type: MessageTypeChat, Sender: "spoofed", Content: " hello "})
	firstChat := readMessage(t, first)
	secondChat := readMessage(t, second)

	for _, message := range []Message{firstChat, secondChat} {
		if message.Type != MessageTypeChat || message.Sender != "alice" || message.Content != "hello" {
			t.Fatalf("unexpected chat message: %+v", message)
		}
		if message.ID != 1 || message.CreatedAt.IsZero() {
			t.Fatalf("saved message metadata missing: %+v", message)
		}
	}

	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.messages) != 1 || repository.messages[0].Sender != "alice" {
		t.Fatalf("unexpected saved messages: %+v", repository.messages)
	}
}

func TestChatRequiresJoin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	handler := NewHandler(hub, &fakeMessageRepository{}, &localPublisher{hub: hub}, logger, []string{"*"})
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWebSocket(t, "ws"+strings.TrimPrefix(server.URL, "http"))
	defer conn.Close()
	writeJSON(t, conn, IncomingMessage{Type: MessageTypeChat, Content: "hello"})
	assertMessage(t, conn, MessageTypeError, "채팅 전에 먼저 입장해야 합니다.")
}

func dialWebSocket(t *testing.T, target string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(target, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func writeJSON(t *testing.T, conn *websocket.Conn, message IncomingMessage) {
	t.Helper()
	if err := conn.WriteJSON(message); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}
}

func readMessage(t *testing.T, conn *websocket.Conn) Message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	var message Message
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("decode websocket message: %v", err)
	}
	return message
}

func assertMessage(t *testing.T, conn *websocket.Conn, messageType, content string) {
	t.Helper()
	message := readMessage(t, conn)
	if message.Type != messageType || message.Content != content {
		t.Fatalf("unexpected message: %+v", message)
	}
}

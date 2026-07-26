package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pang-4/chat/internal/chat"
)

func TestMetricsHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := chat.NewHub(logger)
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()

	metricsHandler(hub).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", contentType)
	}

	body := recorder.Body.String()
	expected := []string{
		"# HELP chat_websocket_connections Current number of active WebSocket connections.",
		"# TYPE chat_websocket_connections gauge",
		"chat_websocket_connections 0",
	}

	for _, value := range expected {
		if !strings.Contains(body, value) {
			t.Errorf("response body does not contain %q:\n%s", value, body)
		}
	}
}

package server

import (
	"net/http"

	"github.com/pang-4/chat/internal/chat"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func metricsHandler(hub *chat.Hub) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "chat",
			Subsystem: "websocket",
			Name:      "connections",
			Help:      "Current number of active WebSocket connections.",
		},
		func() float64 {
			return float64(hub.ConnectionCount())
		},
	))

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

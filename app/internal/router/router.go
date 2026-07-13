package router

import (
	"net/http"

	"github.com/pang-4/chat/internal/handler"
)

func SetupRoutes(){

	http.HandleFunc("/ws", handler.WebSocketHandler)
}
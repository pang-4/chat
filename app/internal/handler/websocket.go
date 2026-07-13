package handler

import(
	"log"
	"net/http"
	"github.com/gorilla/websocket"
	"github.com/pang-4/chat/internal/model"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool{
		return true
	},
}

var hub = model.NewHub()

func init(){
	go hub.Run()
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request){

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil{
		log.Print(err)
		return
	}

	client := &model.Client{
		Conn: conn,
		Send: make(chan model.Message),
	}

	go client.WritePump()
	
	hub.Register <- client

	log.Println("Current Clients : %d\n", len(hub.Clients))
	log.Println("WebSocket Connected")

	client.ReadPump(hub)
}

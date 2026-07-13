package model

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type Client struct {
	Name string
	Conn *websocket.Conn
	Send chan Message
}

func (c *Client) WritePump() {
	for {
		message := <-c.Send

		data, err := json.Marshal(message)
		if err != nil {
			log.Println("[ERROR] Marshal:", err)
			continue
		}

		err = c.Conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Println("[ERROR] Write:", err)
			return
		}
	}
}

func (c *Client) ReadPump(h *Hub) {

	defer func() {

		log.Printf("[LEAVE] %s\n", c.Name)

		h.Broadcast <- NewLeaveMessage(c.Name)
		h.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println("[ERROR] Read:", err)
			break
		}

		var msg Message

		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Println("[ERROR] JSON:", err)
			continue
		}

		if msg.Type == "join" {
			c.Name = msg.Sender
			log.Printf("[JOIN] %s\n", c.Name)
			h.Broadcast <- NewJoinMessage(c.Name)
			continue
		}

		msg.Sender = c.Name

		h.Broadcast <- msg
	}
}
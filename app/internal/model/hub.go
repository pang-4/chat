package model

type Hub struct{
	Clients map[*Client]bool
	Register chan *Client
	Unregister chan *Client
	Broadcast chan Message
}

func NewHub() *Hub{
	return &Hub{
		Clients: make(map[*Client]bool),
		Register: make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast: make(chan Message),
	}
}

func (h *Hub) Run() {
	for {
		select{

		case client := <-h.Register:
			h.Clients[client] =true
		
		case client := <-h.Unregister:
			delete(h.Clients, client)

		case message := <-h.Broadcast:
			for client := range h.Clients {
				client.Send <- message
			}
		}
	}
}
package chat

import "time"

const (
	MessageTypeJoin   = "join"
	MessageTypeChat   = "chat"
	MessageTypeSystem = "system"
	MessageTypeError  = "error"
)

type Message struct {
	ID        int64     `json:"id,omitempty"`
	Type      string    `json:"type"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type IncomingMessage struct {
	Type    string `json:"type"`
	Sender  string `json:"sender"`
	Content string `json:"content"`
}

func NewSystemMessage(content string) Message {
	return Message{
		Type:      MessageTypeSystem,
		Sender:    "SYSTEM",
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
}

func NewErrorMessage(content string) Message {
	return Message{
		Type:      MessageTypeError,
		Sender:    "SYSTEM",
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
}

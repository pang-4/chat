package model

func NewSystemMessage(Content string) Message{
	return Message{
		Type:		"system",
		Sender:		"SYSTEM",
		Content:	Content,
	}
}

func NewJoinMessage(name string) Message{
	return NewSystemMessage(name + "님이 입장했습니다.")
}

func NewLeaveMessage(name string) Message{
	return NewSystemMessage(name + "님이 퇴장했습니다.")
}

func NewChatMessage(sender, content string) Message{
	return Message{
		Type:		"chat",
		Sender:		sender,
		Content:	content,
	}
}
package dto

type SendMessageRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	ReceiverID     string `json:"receiver_id" binding:"required"`
	Content        string `json:"content" binding:"required"`
	MessageType    string `json:"message_type"`
}

type WSMessageFrame struct {
	Event string             `json:"event"` // "send_message", "typing", "mark_read"
	Data  SendMessageRequest `json:"data"`
}

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Message struct {
	ID             bson.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	ConversationID string        `bson:"conversation_id"       json:"conversation_id"`
	SenderID       bson.ObjectID `bson:"sender_id"             json:"sender_id"`
	ReceiverID     bson.ObjectID `bson:"receiver_id"           json:"receiver_id"`
	Content        string        `bson:"content"               json:"content"`
	MessageType    string        `bson:"message_type"          json:"message_type"` // "text", "image"
	Status         string        `bson:"status"                json:"status"`       // "sent", "delivered", "read"
	CreatedAt      time.Time     `bson:"created_at"            json:"created_at"`
}

// chat.go — MongoDB models for friend chat.
//
// Two collections:
//   - chats      → one document per friendship, keyed by chatId
//   - messages   → all messages within a chat (soft-delete via isDeleted flag)
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MessageType enumerates the kinds of content a message can carry.
type MessageType string

const (
	MessageTypeText MessageType = "text"
)

// Chat is stored in the "chats" MongoDB collection.
// One chat document exists per friendship pair.
type Chat struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"    json:"chatId"`
	ChatID        string             `bson:"chatId"           json:"id"`         // deterministic "chat_<uid1>_<uid2>"
	Members       []string           `bson:"members"          json:"members"`    // [userA_hex, userB_hex]
	LastMessage   string             `bson:"lastMessage"      json:"lastMessage"`
	LastMessageAt time.Time          `bson:"lastMessageAt"    json:"lastMessageAt"`
	CreatedAt     time.Time          `bson:"createdAt"        json:"createdAt"`
}

// Message is stored in the "messages" MongoDB collection.
// Messages are NEVER hard-deleted — isDeleted soft-deletes them.
type Message struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"messageId"`
	ChatID    string             `bson:"chatId"        json:"chatId"`
	SenderID  string             `bson:"senderId"      json:"senderId"`
	Type      MessageType        `bson:"type"          json:"type"`
	Text      string             `bson:"text"          json:"text"`      // "" when isDeleted=true
	IsDeleted bool               `bson:"isDeleted"     json:"isDeleted"`
	CreatedAt time.Time          `bson:"createdAt"     json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"     json:"updatedAt"`
}

// DeletedText is the placeholder shown to clients when a message is soft-deleted.
const DeletedText = "This message was deleted"

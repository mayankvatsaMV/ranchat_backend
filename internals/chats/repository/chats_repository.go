package repository

import (
	"context"
	"ranchat/internals/chats/models"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type ChatsRepository interface {
	SaveMessage(context.Context, *models.Message) error
	GetChatHistory(context.Context, string, string, int64) ([]models.Message, error)
	MarkMessagesAsRead(context.Context, string, string) error
}
type MongoChatRepository struct {
	ChatDB *mongo.Collection
}

func NewMongoChatRepository(db *mongo.Database) *MongoChatRepository {
	return &MongoChatRepository{
		ChatDB: db.Collection("chats"),
	}
}

package repository

import (
	"context"
	"ranchat/internals/chats/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ChatsRepository interface {
	SaveMessage(context.Context, *models.Message) error
	GetChatHistory(ctx context.Context, conversationID string, limit int64) ([]models.Message, error)
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

func (mc *MongoChatRepository) SaveMessage(ctx context.Context, msg *models.Message) error {
	_, err := mc.ChatDB.InsertOne(ctx, msg)
	if err != nil {
		return err
	}
	return nil
}

func (mc *MongoChatRepository) GetChatHistory(
	ctx context.Context,
	conversationID string,
	limit int64,
) ([]models.Message, error) {

	filter := bson.M{
		"conversation_id": conversationID,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := mc.ChatDB.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []models.Message

	if err := cursor.All(ctx, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (mc *MongoChatRepository) MarkMessagesAsRead(
	ctx context.Context,
	userID string,
	partnerID string,
) error {

	filter := bson.M{
		"sender_id":   partnerID,
		"receiver_id": userID,
		"is_read":     false,
	}

	update := bson.M{
		"$set": bson.M{
			"is_read": true,
		},
	}

	_, err := mc.ChatDB.UpdateMany(ctx, filter, update)
	return err
}

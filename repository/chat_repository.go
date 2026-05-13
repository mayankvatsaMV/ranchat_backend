// chat_repository.go — MongoDB persistence for friend chats and messages.
//
// Collection: chats
//   - One document per friendship pair
//   - chatId is deterministic: "chat_<smaller_uid>_<larger_uid>"
//
// Collection: messages
//   - Append-only; messages are NEVER removed from MongoDB
//   - Soft-delete: set isDeleted=true and text="" on the document
package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"ranchat_backend/config"
	"ranchat_backend/models"
)

var (
	ErrChatNotFound    = errors.New("chat not found")
	ErrMessageNotFound = errors.New("message not found")
	ErrNotMessageOwner = errors.New("you can only delete your own messages")
)

// ── Collection helpers ────────────────────────────────────────────────────────

func chatCol() *mongo.Collection {
	return config.GetCollection("chats")
}

func messageCol() *mongo.Collection {
	return config.GetCollection("messages")
}

// ── Chat ID generation ────────────────────────────────────────────────────────

// BuildChatID returns a stable, canonical chatId for any two user IDs.
// It always sorts the two IDs so "chat_A_B" == "chat_B_A".
func BuildChatID(uid1, uid2 string) string {
	ids := []string{uid1, uid2}
	sort.Strings(ids)
	return "chat_" + ids[0] + "_" + ids[1]
}

// ── Chats ─────────────────────────────────────────────────────────────────────

// GetOrCreateChat returns the Chat for a friendship, creating it if it does not exist.
// uid1 / uid2 are the hex ObjectID strings of the two friends.
func GetOrCreateChat(ctx context.Context, uid1, uid2 string) (*models.Chat, error) {
	chatID := BuildChatID(uid1, uid2)

	// Try to find existing chat
	existing := &models.Chat{}
	err := chatCol().FindOne(ctx, bson.M{"chatId": chatID}).Decode(existing)
	if err == nil {
		return existing, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Create a new chat document
	now := time.Now().UTC()
	chat := &models.Chat{
		ID:            primitive.NewObjectID(),
		ChatID:        chatID,
		Members:       []string{uid1, uid2},
		LastMessage:   "",
		LastMessageAt: now,
		CreatedAt:     now,
	}
	if _, err := chatCol().InsertOne(ctx, chat); err != nil {
		// Race-condition: another goroutine created it first — just fetch it
		if mongo.IsDuplicateKeyError(err) {
			_ = chatCol().FindOne(ctx, bson.M{"chatId": chatID}).Decode(chat)
			return chat, nil
		}
		return nil, err
	}
	return chat, nil
}

// GetChatByID returns a chat document by its MongoDB _id.
func GetChatByID(ctx context.Context, id primitive.ObjectID) (*models.Chat, error) {
	chat := &models.Chat{}
	err := chatCol().FindOne(ctx, bson.M{"_id": id}).Decode(chat)
	if err == mongo.ErrNoDocuments {
		return nil, ErrChatNotFound
	}
	return chat, err
}

// GetChatByChatID returns a chat document by its string chatId.
func GetChatByChatID(ctx context.Context, chatID string) (*models.Chat, error) {
	chat := &models.Chat{}
	err := chatCol().FindOne(ctx, bson.M{"chatId": chatID}).Decode(chat)
	if err == mongo.ErrNoDocuments {
		return nil, ErrChatNotFound
	}
	return chat, err
}

// GetChatsForUser returns all chats where the user is a member, sorted newest-first.
func GetChatsForUser(ctx context.Context, userID string) ([]models.Chat, error) {
	opts := options.Find().SetSort(bson.D{{Key: "lastMessageAt", Value: -1}})
	cursor, err := chatCol().Find(ctx, bson.M{"members": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chats []models.Chat
	if err := cursor.All(ctx, &chats); err != nil {
		return nil, err
	}
	return chats, nil
}

// updateChatLastMessage sets the lastMessage / lastMessageAt fields on a chat.
func updateChatLastMessage(ctx context.Context, chatID, text string) {
	_, _ = chatCol().UpdateOne(ctx,
		bson.M{"chatId": chatID},
		bson.M{"$set": bson.M{
			"lastMessage":   text,
			"lastMessageAt": time.Now().UTC(),
		}},
	)
}

// ── Messages ──────────────────────────────────────────────────────────────────

// SendMessage persists a new text message and updates the chat's lastMessage.
func SendMessage(ctx context.Context, chatID, senderID, text string) (*models.Message, error) {
	now := time.Now().UTC()
	msg := &models.Message{
		ID:        primitive.NewObjectID(),
		ChatID:    chatID,
		SenderID:  senderID,
		Type:      models.MessageTypeText,
		Text:      text,
		IsDeleted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := messageCol().InsertOne(ctx, msg); err != nil {
		return nil, err
	}

	// Best-effort update of the chat summary (non-blocking is fine here)
	go updateChatLastMessage(context.Background(), chatID, text)

	return msg, nil
}

// SoftDeleteMessage marks a message as deleted (sets isDeleted=true, text="").
// Only the original sender can delete their message.
func SoftDeleteMessage(ctx context.Context, messageID primitive.ObjectID, requesterID string) (*models.Message, error) {
	msg := &models.Message{}
	if err := messageCol().FindOne(ctx, bson.M{"_id": messageID}).Decode(msg); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}

	if msg.SenderID != requesterID {
		return nil, ErrNotMessageOwner
	}

	now := time.Now().UTC()
	_, err := messageCol().UpdateOne(ctx,
		bson.M{"_id": messageID},
		bson.M{"$set": bson.M{
			"isDeleted": true,
			"text":      "",
			"updatedAt": now,
		}},
	)
	if err != nil {
		return nil, err
	}

	msg.IsDeleted = true
	msg.Text = ""
	msg.UpdatedAt = now
	return msg, nil
}

// GetMessages returns paginated messages for a chat, sorted oldest-first.
// Pass beforeID="" to get the latest page.
// Pass beforeID=<messageId> to get the page before that message (cursor-based pagination).
func GetMessages(ctx context.Context, chatID string, limit int64, beforeID string) ([]models.Message, error) {
	filter := bson.M{"chatId": chatID}

	if beforeID != "" {
		oid, err := primitive.ObjectIDFromHex(beforeID)
		if err == nil {
			filter["_id"] = bson.M{"$lt": oid}
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}). // newest first from DB
		SetLimit(limit)

	cursor, err := messageCol().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var msgs []models.Message
	if err := cursor.All(ctx, &msgs); err != nil {
		return nil, err
	}

	// Reverse so the result is chronological (oldest → newest)
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// GetMessageByID fetches a single message by its MongoDB ObjectID.
func GetMessageByID(ctx context.Context, messageID primitive.ObjectID) (*models.Message, error) {
	msg := &models.Message{}
	if err := messageCol().FindOne(ctx, bson.M{"_id": messageID}).Decode(msg); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	return msg, nil
}

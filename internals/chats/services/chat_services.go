package services

import (
	"context"
	"errors"
	"time"

	"ranchat/internals/chats/dto"
	"ranchat/internals/chats/models"
	"ranchat/internals/chats/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ChatService struct {
	chatRepo repository.ChatsRepository
}

func NewChatService(chatRepo repository.ChatsRepository) *ChatService {
	return &ChatService{
		chatRepo: chatRepo,
	}
}

func (cs *ChatService) SendMessage(
	ctx context.Context,
	senderID string,
	req *dto.SendMessageRequest,
) (*models.Message, error) {

	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if req.ReceiverID == "" {
		return nil, errors.New("receiver_id is required")
	}

	if req.Content == "" {
		return nil, errors.New("content is required")
	}

	senderObjectID, err := bson.ObjectIDFromHex(senderID)
	if err != nil {
		return nil, errors.New("invalid sender_id")
	}

	receiverObjectID, err := bson.ObjectIDFromHex(req.ReceiverID)
	if err != nil {
		return nil, errors.New("invalid receiver_id")
	}

	ConversationID := generateParticipantID(
		senderID,
		req.ReceiverID,
	)

	messageType := req.MessageType

	if messageType == "" {
		messageType = "text"
	}

	message := &models.Message{
		ID:             bson.NewObjectID(),
		ConversationID: ConversationID,
		SenderID:       senderObjectID,
		ReceiverID:     receiverObjectID,
		Content:        req.Content,
		MessageType:    messageType,
		Status:         "sent",
		CreatedAt:      time.Now(),
	}

	if err := cs.chatRepo.SaveMessage(ctx, message); err != nil {
		return nil, err
	}

	return message, nil
}
func (cs *ChatService) GetChatHistory(
	ctx context.Context,
	userID string,
	receiverID string,
	limit int64,
) ([]models.Message, error) {

	if userID == "" || receiverID == "" {
		return nil, errors.New("user_id and receiver_id are required")
	}

	if limit <= 0 {
		limit = 50
	}

	if limit > 100 {
		limit = 100
	}

	conversationID := generateParticipantID(userID, receiverID)

	return cs.chatRepo.GetChatHistory(
		ctx,
		conversationID,
		limit,
	)
}

func (cs *ChatService) MarkMessagesAsRead(
	ctx context.Context,
	userID string,
	receiverID string,
) error {

	if userID == "" || receiverID == "" {
		return errors.New("user_id and receiver_id are required")
	}

	return cs.chatRepo.MarkMessagesAsRead(
		ctx,
		userID,
		receiverID,
	)
}

func generateParticipantID(userA, userB string) string {
	if userA < userB {
		return userA + "_" + userB
	}

	return userB + "_" + userA
}

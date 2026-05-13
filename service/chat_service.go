// chat_service.go — Business logic for friend chat.
//
// Bridges:
//   - repository.chat_repository       (MongoDB)
//   - repository.chat_room_repository  (in-memory WebSocket rooms)
//   - repository.friendship_repository (access control)
package service

import (
	"context"
	"errors"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"ranchat_backend/models"
	"ranchat_backend/repository"
)

var (
	ErrNotFriends  = errors.New("you are not friends with this user")
	ErrNotInChat   = errors.New("you are not a member of this chat")
	ErrEmptyText   = errors.New("message text cannot be empty")
	ErrMsgTooLong  = errors.New("message text exceeds maximum length (2000 chars)")
)

const maxMessageLength = 2000

// GetOrOpenChat returns (or creates) the Chat document for two friends.
// Validates that a friendship actually exists before creating.
func GetOrOpenChat(ctx context.Context, callerID, friendID string) (*models.Chat, int, error) {
	callerOID, err := primitive.ObjectIDFromHex(callerID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId")
	}
	friendOID, err := primitive.ObjectIDFromHex(friendID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid friendId")
	}

	// Guard: must be actual friends
	ok, err := repository.AreFriends(ctx, callerOID, friendOID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if !ok {
		return nil, http.StatusForbidden, ErrNotFriends
	}

	chat, err := repository.GetOrCreateChat(ctx, callerID, friendID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return chat, http.StatusOK, nil
}

// GetMyChats returns all chats for the calling user, sorted newest-first.
func GetMyChats(ctx context.Context, userID string) ([]models.Chat, int, error) {
	chats, err := repository.GetChatsForUser(ctx, userID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return chats, http.StatusOK, nil
}

// GetChatMessages returns paginated messages for a chat.
// beforeID: cursor for older-page loading ("" = latest page).
// limit: number of messages per page (capped to 100).
func GetChatMessages(ctx context.Context, callerID, chatID string, limit int64, beforeID string) ([]models.Message, int, error) {
	chat, err := repository.GetChatByChatID(ctx, chatID)
	if err != nil {
		if err == repository.ErrChatNotFound {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}

	// Access control: caller must be a member of this chat
	if !isMember(chat.Members, callerID) {
		return nil, http.StatusForbidden, ErrNotInChat
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	msgs, err := repository.GetMessages(ctx, chatID, limit, beforeID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// Mask deleted messages for the client
	for i := range msgs {
		if msgs[i].IsDeleted {
			msgs[i].Text = models.DeletedText
		}
	}
	return msgs, http.StatusOK, nil
}

// SendChatMessage persists a message and pushes a real-time event to both parties.
func SendChatMessage(ctx context.Context, callerID, chatID, text string) (*models.Message, int, error) {
	if text == "" {
		return nil, http.StatusBadRequest, ErrEmptyText
	}
	if len(text) > maxMessageLength {
		return nil, http.StatusBadRequest, ErrMsgTooLong
	}

	// Access control
	chat, err := repository.GetChatByChatID(ctx, chatID)
	if err != nil {
		if err == repository.ErrChatNotFound {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}
	if !isMember(chat.Members, callerID) {
		return nil, http.StatusForbidden, ErrNotInChat
	}

	msg, err := repository.SendMessage(ctx, chatID, callerID, text)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	// Real-time delivery — broadcast to all connected room members
	repository.BroadcastToChat(chatID, models.WSMessage{
		Type: models.WSEventNewMessage,
		Data: models.WSChatMessagePayload{
			MessageID: msg.ID.Hex(),
			ChatID:    chatID,
			SenderID:  callerID,
			Text:      text,
			CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})

	return msg, http.StatusCreated, nil
}

// DeleteChatMessage soft-deletes a message and notifies both parties.
func DeleteChatMessage(ctx context.Context, callerID, messageID string) (*models.Message, int, error) {
	msgOID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid messageId")
	}

	msg, err := repository.SoftDeleteMessage(ctx, msgOID, callerID)
	if err != nil {
		switch err {
		case repository.ErrMessageNotFound:
			return nil, http.StatusNotFound, err
		case repository.ErrNotMessageOwner:
			return nil, http.StatusForbidden, err
		default:
			return nil, http.StatusInternalServerError, err
		}
	}

	// Broadcast deletion event so the partner's UI can update instantly
	repository.BroadcastToChat(msg.ChatID, models.WSMessage{
		Type: models.WSEventMessageDeleted,
		Data: models.WSDeleteMessagePayload{
			MessageID: msg.ID.Hex(),
			ChatID:    msg.ChatID,
		},
	})

	// Mask deleted text before returning
	msg.Text = models.DeletedText
	return msg, http.StatusOK, nil
}

// isMember checks whether userID is in the members slice.
func isMember(members []string, userID string) bool {
	for _, m := range members {
		if m == userID {
			return true
		}
	}
	return false
}

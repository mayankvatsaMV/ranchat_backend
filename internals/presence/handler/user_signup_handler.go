package handler

import (
	"context"
	"log"
	"ranchat/events"
	presence "ranchat/internals/presence/services"
)

type UserSignupHandler struct {
	PresenceService *presence.PresenceService
}

func (h *UserSignupHandler) Handle(payload any) error {
	event, ok := payload.(events.UserSignedUpEvent)
	if !ok {
		return nil
	}

	_, err := h.PresenceService.UpsertPresence(
		context.Background(),
		event.UserID,
		event.DeviceID,
	)
	if err != nil {
		log.Printf("failed to initialize presence: %v", err)
		return err // 👈 Returning err triggers retries in Publish!
	}
	return nil
}

package service

import (
	"context"
	"ranchat/internals/presence/dto"
	"ranchat/internals/presence/models"
	"ranchat/internals/presence/repository"
	"time"
)

type PresenceService struct {
	Repo repository.PresenceRepository
}

func NewPresenceService(repo repository.PresenceRepository) *PresenceService {
	return &PresenceService{Repo: repo}
}

func (s *PresenceService) UpsertPresence(ctx context.Context, userId, deviceId string) (*models.Presence, error) {
	// Initialize model
	p := &models.Presence{
		UserID:       userId,
		DeviceID:     deviceId,
		LastActiveAt: time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Pass to repo for persistence (SavePresence uses atomic SetNX)
	if err := s.Repo.SavePresence(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PresenceService) HeartBeat(ctx context.Context, userId string) error {
	if err := s.Repo.Heartbeat(ctx, userId); err != nil {
		return err
	}
	return nil
}
func (s *PresenceService) UpdatePresence(ctx context.Context, userId string, updateData dto.UpdatePresenceFields) (*models.Presence, error) {

	if p, err := s.Repo.UpdatePresence(ctx, userId, updateData); err != nil {
		return nil, err
	} else {
		return p, nil
	}
}

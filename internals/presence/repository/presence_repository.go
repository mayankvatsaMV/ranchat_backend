package repository

import (
	"context"
	"ranchat/errors"
	"ranchat/internals/presence/database"
	"ranchat/internals/presence/dto"
	"ranchat/internals/presence/models"
	"time"
)

type PresenceRepository interface {
	SavePresence(ctx context.Context, p *models.Presence) error
	GetPresence(ctx context.Context, userId string) (*models.Presence, error)
	UpdatePresence(ctx context.Context, userId string, updateFields dto.UpdatePresenceFields) (*models.Presence, error)
	Heartbeat(ctx context.Context, userId string) error
}

type InMemoryPresenceRepository struct {
	DB *database.PresenceDB
}

func NewInMemoryPresenceRepository(db *database.PresenceDB) *InMemoryPresenceRepository {
	return &InMemoryPresenceRepository{DB: db}
}

func (r *InMemoryPresenceRepository) SavePresence(ctx context.Context, p *models.Presence) error {
	r.DB.Mu.Lock()
	defer r.DB.Mu.Unlock()

	if _, ok := r.DB.Presences[p.UserID]; ok {
		return errors.ErrPresenceExists
	}
	r.DB.Presences[p.UserID] = p
	return nil
}
func (r *InMemoryPresenceRepository) GetPresence(ctx context.Context, userId string) (*models.Presence, error) {
	r.DB.Mu.RLock()
	defer r.DB.Mu.RUnlock()

	p, exists := r.DB.Presences[userId]
	if !exists {
		return nil, errors.ErrPresenceNotFound
	}

	// Return a copy to avoid exposing internal pointer
	copy := *p
	return &copy, nil
}

// func (r *InMemoryPresenceRepository) UpsertPresence(ctx context.Context, userId, deviceId string) (*models.Presence, error) {
// 	r.DB.Mu.Lock()
// 	defer r.DB.Mu.Unlock()
// 	if _, ok := r.DB.Presences[userId]; ok {
// 		return nil, errors.ErrPresenceExists
// 	} else {
// 		p := &models.Presence{
// 			UserID:         userId,
// 			DeviceID:       deviceId,
// 			LastActiveAt:   time.Now(),
// 			IsSearching:    false,
// 			IsMatched:      false,
// 			CurrentMatchID: nil,
// 			ActiveChatID:   nil,
// 			TypingTo:       nil,
// 			UpdatedAt:      time.Now(),
// 		}
// 		r.DB.Presences[userId] = p
// 		return p, nil
// 	}

// }

func (r *InMemoryPresenceRepository) Heartbeat(ctx context.Context, userId string) error {
	r.DB.Mu.Lock()
	defer r.DB.Mu.Unlock()
	p, exists := r.DB.Presences[userId]
	if !exists {
		return errors.ErrPresenceNotFound
	}

	now := time.Now()
	p.LastActiveAt = now
	p.UpdatedAt = now
	return nil
}

func (r *InMemoryPresenceRepository) UpdatePresence(ctx context.Context, userId string, updateFileds dto.UpdatePresenceFields) (*models.Presence, error) {
	r.DB.Mu.Lock()
	defer r.DB.Mu.Unlock()
	p, exists := r.DB.Presences[userId]
	if !exists {
		return nil, errors.ErrPresenceNotFound
	}
	if updateFileds.TypingTo != nil {
		p.TypingTo = updateFileds.TypingTo
	}
	if updateFileds.ActiveChatID != nil {
		p.ActiveChatID = updateFileds.ActiveChatID
	}
	// if updateFileds.IsSearching != nil {
	// 	p.IsSearching = *updateFileds.IsSearching
	// }
	p.UpdatedAt = time.Now()
	copy := *r.DB.Presences[userId]
	return &copy, nil
}

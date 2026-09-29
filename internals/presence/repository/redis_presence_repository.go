package repository

import (
	"context"
	"encoding/json"
	"time"

	errors "ranchat"
	"ranchat/internals/presence/dto"
	"ranchat/internals/presence/models"

	"github.com/redis/go-redis/v9"
)

type RedisPresenceRepository struct {
	client *redis.Client
}

func NewRedisPresenceRepository(
	client *redis.Client,
) *RedisPresenceRepository {
	return &RedisPresenceRepository{
		client: client,
	}
}

func presenceKey(userID string) string {
	return "presence:" + userID
}

func (r *RedisPresenceRepository) SavePresence(
	ctx context.Context,
	p *models.Presence,
) error {

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	created, err := r.client.SetNX(
		ctx,
		presenceKey(p.UserID),
		data,
		0,
	).Result()

	if err != nil {
		return err
	}

	if !created {
		return errors.ErrPresenceExists
	}

	return nil
}

func (r *RedisPresenceRepository) GetPresence(
	ctx context.Context,
	userID string,
) (*models.Presence, error) {

	data, err := r.client.Get(
		ctx,
		presenceKey(userID),
	).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, errors.ErrPresenceNotFound
		}

		return nil, err
	}

	var presence models.Presence

	if err := json.Unmarshal(
		[]byte(data),
		&presence,
	); err != nil {
		return nil, err
	}

	return &presence, nil
}

func (r *RedisPresenceRepository) Heartbeat(
	ctx context.Context,
	userID string,
) error {

	presence, err := r.GetPresence(ctx, userID)
	if err != nil {
		return err
	}

	now := time.Now()

	presence.LastActiveAt = now
	presence.UpdatedAt = now

	data, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	return r.client.Set(
		ctx,
		presenceKey(userID),
		data,
		0,
	).Err()
}

func (r *RedisPresenceRepository) UpdatePresence(
	ctx context.Context,
	userID string,
	updateFields dto.UpdatePresenceFields,
) (*models.Presence, error) {

	p, err := r.GetPresence(ctx, userID)
	if err != nil {
		return nil, err
	}

	if updateFields.TypingTo != nil {
		p.TypingTo = updateFields.TypingTo
	}

	if updateFields.ActiveChatID != nil {
		p.ActiveChatID = updateFields.ActiveChatID
	}

	// if updateFields.IsSearching != nil {
	// 	p.IsSearching = *updateFields.IsSearching
	// }

	p.UpdatedAt = time.Now()

	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	if err := r.client.Set(
		ctx,
		presenceKey(userID),
		data,
		0,
	).Err(); err != nil {
		return nil, err
	}

	return p, nil
}

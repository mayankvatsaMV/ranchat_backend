package database

import (
	"ranchat/internals/presence/models"
	"sync"
)

type PresenceDB struct {
	Mu        sync.RWMutex
	Presences map[string]*models.Presence
}

func NewPresenceDB() *PresenceDB {
	return &PresenceDB{
		Presences: make(map[string]*models.Presence),
	}
}

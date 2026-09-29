package database

import (
	"sync"

	"ranchat/internals/match_making/dto"
	"ranchat/internals/match_making/models"
)

type MatchmakingMemoryDB struct {
	Mu       sync.RWMutex
	Requests map[string]*dto.MatchmakingRequest
}

func NewMatchmakingMemoryDB() *MatchmakingMemoryDB {
	return &MatchmakingMemoryDB{
		Requests: make(map[string]*dto.MatchmakingRequest),
	}
}

type MatchMemoryDB struct {
	Mu      sync.RWMutex
	Matches map[string]*models.Match
}

func NewMatchMemoryDB() *MatchMemoryDB {
	return &MatchMemoryDB{
		Matches: make(map[string]*models.Match),
	}
}

type EnqueueChan struct {
	Mu    sync.RWMutex
	Chans map[string]chan *models.Match
}

func NewEnqueueChan() *EnqueueChan {
	return &EnqueueChan{
		Chans: make(map[string]chan *models.Match),
	}
}

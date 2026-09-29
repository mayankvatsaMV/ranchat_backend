package repository

import (
	"context"
	"time"

	"ranchat/internals/match_making/database"
	"ranchat/internals/match_making/dto"
	"ranchat/internals/match_making/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type MatchMakingRepository struct {
	DB      *database.MatchmakingMemoryDB
	MatchDB *database.MatchMemoryDB
}

func NewMatchMakingRepository(
	db *database.MatchmakingMemoryDB,
	matchDB *database.MatchMemoryDB,
) *MatchMakingRepository {

	return &MatchMakingRepository{
		DB:      db,
		MatchDB: matchDB,
	}
}

func (r *MatchMakingRepository) FindMatch(
	ctx context.Context,
	req dto.MatchmakingRequest,
) *dto.MatchmakingRequest {

	r.DB.Mu.RLock()
	defer r.DB.Mu.RUnlock()

	for _, candidate := range r.DB.Requests {

		if candidate.UserID == req.UserID {
			continue
		}

		if candidate.Interest != req.Interest {
			continue
		}

		if candidate.Gender != req.GenderPref {
			continue
		}

		if candidate.GenderPref != req.Gender {
			continue
		}

		return candidate
	}

	return nil
}

func (r *MatchMakingRepository) AddRequest(
	ctx context.Context,
	req dto.MatchmakingRequest,
) {

	r.DB.Mu.Lock()
	defer r.DB.Mu.Unlock()

	r.DB.Requests[req.UserID] = &req
}
func (r *MatchMakingRepository) RemoveRequest(
	ctx context.Context,
	userID string,
) {
	r.DB.Mu.Lock()
	defer r.DB.Mu.Unlock()

	delete(r.DB.Requests, userID)
}
func (r *MatchMakingRepository) CreateMatch(
	ctx context.Context,
	currentUser dto.MatchmakingRequest,
	candidate dto.MatchmakingRequest,
) *models.Match {

	match := &models.Match{
		MatchID: bson.NewObjectID().Hex(),
		UserIDs: [2]string{
			currentUser.UserID,
			candidate.UserID,
		},
		CreatedAt: time.Now(),
	}

	r.MatchDB.Mu.Lock()
	defer r.MatchDB.Mu.Unlock()

	r.MatchDB.Matches[match.MatchID] = match

	return match
}

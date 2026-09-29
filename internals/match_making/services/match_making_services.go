package service

import (
	"context"

	"ranchat/internals/match_making/database"
	"ranchat/internals/match_making/dto"
	"ranchat/internals/match_making/models"
	"ranchat/internals/match_making/repository"
)

type MatchMakingService struct {
	Repo        *repository.MatchMakingRepository
	EnqueueChan *database.EnqueueChan
}

func (s *MatchMakingService) MatchAndEnqueue(
	ctx context.Context,
	matchReq dto.MatchmakingRequest,
) chan *models.Match {

	candidate := s.Repo.FindMatch(ctx, matchReq)

	if candidate == nil {
		ch := make(chan *models.Match, 1)

		s.EnqueueChan.Mu.Lock()
		s.EnqueueChan.Chans[matchReq.UserID] = ch
		s.EnqueueChan.Mu.Unlock()
		s.Repo.AddRequest(ctx, matchReq)

		return ch
	}

	match := s.Repo.CreateMatch(
		ctx,
		matchReq,
		*candidate,
	)

	s.Repo.RemoveRequest(
		ctx,
		candidate.UserID,
	)

	s.EnqueueChan.Mu.RLock()
	ch, exists := s.EnqueueChan.Chans[candidate.UserID]
	s.EnqueueChan.Mu.RUnlock()

	if exists {

		ch <- match

		s.EnqueueChan.Mu.Lock()
		delete(s.EnqueueChan.Chans, candidate.UserID)
		s.EnqueueChan.Mu.Unlock()
	}

	currentUserCh := make(chan *models.Match, 1)
	currentUserCh <- match

	return currentUserCh
}
func (s *MatchMakingService) CancelMatching(userID string) {

	s.Repo.RemoveRequest(
		context.Background(),
		userID,
	)

	s.EnqueueChan.Mu.Lock()
	delete(s.EnqueueChan.Chans, userID)
	s.EnqueueChan.Mu.Unlock()
}

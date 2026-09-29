package services

import (
	"context"

	"ranchat/internals/friendship/models"
	"ranchat/internals/friendship/repository"
)

type FriendshipService struct {
	Repo repository.FriendshipRepository
}

// Send friend request
func (fs *FriendshipService) SendFriendRequest(
	ctx context.Context,
	friendRequest models.FriendRequest,
) (models.FriendRequest, error) {

	return fs.Repo.SendFriendRequest(ctx, friendRequest)
}

// Accept friend request
func (fs *FriendshipService) AcceptFriendRequest(
	ctx context.Context,
	requestID string,
	userID string,
) (*models.Friend, error) {

	err, friendship := fs.Repo.AcceptFriendRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return friendship, nil
}

func (fs *FriendshipService) DeclineFriendRequest(
	ctx context.Context,
	requestID string,
) error {

	return fs.Repo.DeclineFriendRequest(ctx, requestID)
}

func (fs *FriendshipService) RemoveFriend(
	ctx context.Context,
	friendID string,
) error {

	return fs.Repo.RemoveFriend(ctx, friendID)
}

func (fs *FriendshipService) GetAllFriendRequests(
	ctx context.Context,
	userID string,
) ([]models.FriendRequest, error) {

	err, requests := fs.Repo.GetAllFriendRequest(ctx, userID)

	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (fs *FriendshipService) GetAllFriends(
	ctx context.Context,
	userID string,
) ([]models.Friend, error) {

	err, friends := fs.Repo.GetAllFriends(ctx, userID)

	if err != nil {
		return nil, err
	}

	return friends, nil
}

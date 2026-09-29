package repository

import (
	"context"
	"fmt"
	"ranchat/internals/friendship/models"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type FriendshipRepository interface {
	SendFriendRequest(context.Context, models.FriendRequest) (models.FriendRequest, error) //done
	AcceptFriendRequest(context.Context, string) (error, *models.Friend)                   //done
	DeclineFriendRequest(context.Context, string) error
	RemoveFriend(context.Context, string) error
	GetAllFriendRequest(context.Context, string) (error, []models.FriendRequest)
	GetAllFriends(context.Context, string) (error, []models.Friend)
}

type MongoFriendshipRepository struct {
	FriendDB        *mongo.Collection
	FriendRequestDB *mongo.Collection
}

func NewMongoFriendshipRepository(db *mongo.Database) *MongoFriendshipRepository {
	return &MongoFriendshipRepository{
		FriendDB:        db.Collection("friends"),
		FriendRequestDB: db.Collection("friendrequests"),
	}
}

func (f *MongoFriendshipRepository) SendFriendRequest(
	ctx context.Context,
	requestObj models.FriendRequest,
) (models.FriendRequest, error) {

	filter := bson.M{
		"sender_id":   requestObj.SenderID,
		"receiver_id": requestObj.ReceiverID,
	}

	var existingRequest models.FriendRequest

	err := f.FriendRequestDB.FindOne(ctx, filter).Decode(&existingRequest)

	if err == nil {
		return models.FriendRequest{}, fmt.Errorf("friend request already exists")
	}

	if err != mongo.ErrNoDocuments {
		return models.FriendRequest{}, err
	}

	result, err := f.FriendRequestDB.InsertOne(ctx, requestObj)
	if err != nil {
		return models.FriendRequest{}, err
	}

	requestObj.ID = result.InsertedID.(bson.ObjectID)

	return requestObj, nil
}
func (f *MongoFriendshipRepository) AcceptFriendRequest(ctx context.Context, requestId string) (error, *models.Friend) {
	rId, err := bson.ObjectIDFromHex(requestId)
	if err != nil {
		return err, nil
	}
	filter := bson.M{
		"_id": rId,
	}
	res := f.FriendRequestDB.FindOne(ctx, filter)

	var friendreq models.FriendRequest
	err = res.Decode(&friendreq)
	if err != nil {
		return err, nil
	}
	friendship := models.Friend{
		ID: bson.NewObjectID(),
		UserIDs: []bson.ObjectID{
			friendreq.SenderID,
			friendreq.ReceiverID,
		},
		CreatedAt: time.Now(),
	}
	_, err = f.FriendDB.InsertOne(ctx, friendship)
	if err != nil {
		return err, nil
	}
	_, err = f.FriendRequestDB.DeleteOne(ctx, filter)
	if err != nil {
		return err, nil
	}

	return nil, &friendship
}

func (f *MongoFriendshipRepository) DeclineFriendRequest(ctx context.Context, requestId string) error {
	rId, err := bson.ObjectIDFromHex(requestId)
	if err != nil {
		return err
	}
	filter := bson.M{
		"_id": rId,
	}
	_, err = f.FriendRequestDB.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil

}

func (f *MongoFriendshipRepository) RemoveFriend(ctx context.Context, friendId string) error {
	fId, err := bson.ObjectIDFromHex(friendId)
	if err != nil {
		return err
	}
	filter := bson.M{
		"_id": fId,
	}
	_, err = f.FriendDB.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (f *MongoFriendshipRepository) GetAllFriends(
	ctx context.Context,
	userId string,
) (error, []models.Friend) {

	uID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return err, nil
	}

	filter := bson.M{
		"user_ids": uID,
	}

	cursor, err := f.FriendDB.Find(ctx, filter)
	if err != nil {
		return err, nil
	}
	defer cursor.Close(ctx)

	var friends []models.Friend

	if err := cursor.All(ctx, &friends); err != nil {
		return err, nil
	}

	return nil, friends
}
func (f *MongoFriendshipRepository) GetAllFriendRequest(
	ctx context.Context,
	userID string,
) (error, []models.FriendRequest) {

	uID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return err, nil
	}

	filter := bson.M{
		"receiver_id": uID,
	}

	cursor, err := f.FriendRequestDB.Find(ctx, filter)
	if err != nil {
		return err, nil
	}
	defer cursor.Close(ctx)

	var requests []models.FriendRequest

	if err := cursor.All(ctx, &requests); err != nil {
		return err, nil
	}

	return nil, requests
}

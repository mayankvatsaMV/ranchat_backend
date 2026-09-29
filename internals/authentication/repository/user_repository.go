package repository

import (
	"context"
	errors "ranchat"
	"ranchat/internals/authentication/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user models.User) error
	UpdateUser(ctx context.Context, user models.User) (models.User, error)
	GetUserById(ctx context.Context, id string) (models.User, error)
}

type MongoUserRepo struct {
	collection *mongo.Collection
}

func NewMongoUserRepo(db *mongo.Database) *MongoUserRepo {
	return &MongoUserRepo{
		collection: db.Collection("users"),
	}
}

func (repo *MongoUserRepo) CreateUser(ctx context.Context, user models.User) error {
	_, err := repo.collection.InsertOne(ctx, user)
	if err != nil {
		// Handle duplicate key error
		if mongo.IsDuplicateKeyError(err) {
			return errors.ErrDuplicateUser
		}
		return errors.ErrDBFailure
	}
	return nil
}

func (repo *MongoUserRepo) UpdateUser(ctx context.Context, user models.User) (models.User, error) {
	filter := bson.M{"_id": user.UserId}
	update := bson.M{"$set": user}

	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	result := repo.collection.FindOneAndUpdate(ctx, filter, update, opt)

	updated := models.User{}
	if err := result.Decode(&updated); err != nil {
		if err == mongo.ErrNoDocuments {
			return models.User{}, errors.ErrUserNotFound
		}
		return models.User{}, errors.ErrDBFailure
	}

	return updated, nil
}

func (repo *MongoUserRepo) GetUserById(ctx context.Context, id string) (models.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, errors.ErrUserNotFound
	}

	filter := bson.M{"_id": objectID}
	result := repo.collection.FindOne(ctx, filter)

	var user models.User
	if err := result.Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return models.User{}, errors.ErrUserNotFound
		}
		return models.User{}, errors.ErrDBFailure
	}
	return user, nil
}

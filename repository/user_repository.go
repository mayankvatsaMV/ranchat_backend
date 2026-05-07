// Package repository handles all direct MongoDB reads and writes for users.
// No business logic lives here — just raw database operations.
// The service layer calls these functions and decides what to do with the results.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"ranchat_backend/config"
	"ranchat_backend/models"
)

// col is a small helper so we don't repeat config.GetCollection("users") everywhere.
func col() *mongo.Collection {
	return config.GetCollection("users")
}

// CreateUser inserts a brand-new user document and stamps the system timestamps.
func CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	user.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.LastSeen = now

	if _, err := col().InsertOne(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// FindUserByID looks up a user by their MongoDB ObjectID.
// Returns mongo.ErrNoDocuments if the user does not exist.
func FindUserByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := col().FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByDeviceID looks up a user by the device identifier sent from the client.
// Returns mongo.ErrNoDocuments if no account exists for that device yet.
func FindUserByDeviceID(ctx context.Context, deviceID string) (*models.User, error) {
	var user models.User
	err := col().FindOne(ctx, bson.M{"deviceId": deviceID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// TouchLastSeen updates the lastSeen and updatedAt timestamps.
// Called every time an existing user logs in.
func TouchLastSeen(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now().UTC()
	_, err := col().UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"lastSeen": now, "updatedAt": now}},
	)
	return err
}

// UpdateUser applies only the provided fields (partial update / PATCH semantics).
// It always stamps updatedAt, then returns the fresh document from the database.
func UpdateUser(ctx context.Context, id primitive.ObjectID, fields bson.M) (*models.User, error) {
	fields["updatedAt"] = time.Now().UTC()

	if _, err := col().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields}); err != nil {
		return nil, err
	}

	// Return the updated document so the caller always has fresh data
	return FindUserByID(ctx, id)
}

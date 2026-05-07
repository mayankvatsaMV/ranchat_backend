// user_service.go — business logic for user accounts.
package service

import (
	"context"
	"errors"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"ranchat_backend/models"
	"ranchat_backend/repository"
	"ranchat_backend/utils"
)

// AnonymousLogin registers a new user OR logs in an existing one — same call, same deviceId.
func AnonymousLogin(ctx context.Context, req *models.AnonymousLoginRequest) (*models.AuthResponse, int, error) {
	existing, err := repository.FindUserByDeviceID(ctx, req.DeviceID)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, http.StatusInternalServerError, err
	}

	var user *models.User

	if existing != nil {
		_ = repository.TouchLastSeen(ctx, existing.ID)
		user = existing
	} else {
		if req.Interests == nil {
			req.Interests = []string{}
		}
		created, err := repository.CreateUser(ctx, &models.User{
			FullName:  req.FullName,
			Gender:    req.Gender,
			Age:       req.Age,
			About:     req.About,
			Interests: req.Interests,
			DeviceID:  req.DeviceID,
		})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		user = created
	}

	presence, err := repository.UpsertPresence(ctx, user.ID, user.DeviceID)
	if err != nil {
		presence = nil
		_ = err
	}

	token, expiry, err := utils.GenerateToken(user.ID.Hex(), user.DeviceID)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("could not generate token: " + err.Error())
	}

	status := http.StatusOK
	if existing == nil {
		status = http.StatusCreated
	}

	return &models.AuthResponse{
		Token:     token,
		ExpiresAt: utils.FormatTime(expiry),
		User:      user,
		Presence:  presence,
	}, status, nil
}

// GetUserByID fetches any user's profile by their ID string.
func GetUserByID(ctx context.Context, userID string) (*models.User, int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId format")
	}

	user, err := repository.FindUserByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, http.StatusNotFound, errors.New("user not found")
		}
		return nil, http.StatusInternalServerError, err
	}

	return user, http.StatusOK, nil
}

// UpdateUser applies only the fields the client sent (PATCH semantics).
func UpdateUser(ctx context.Context, userID string, req *models.UpdateUserRequest) (*models.User, int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId")
	}

	fields := bson.M{}
	if req.FullName != nil {
		fields["fullName"] = *req.FullName
	}
	if req.Gender != nil {
		fields["gender"] = *req.Gender
	}
	if req.Age != nil {
		fields["age"] = *req.Age
	}
	if req.About != nil {
		fields["about"] = *req.About
	}
	if req.Interests != nil {
		fields["interests"] = *req.Interests
	}

	if len(fields) == 0 {
		return nil, http.StatusBadRequest, errors.New("no fields provided to update")
	}

	updated, err := repository.UpdateUser(ctx, oid, fields)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return updated, http.StatusOK, nil
}

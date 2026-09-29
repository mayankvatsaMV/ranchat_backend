package service

import (
	"context"
	errors "ranchat"
	"ranchat/events"
	"ranchat/internals/authentication/dto"
	"ranchat/internals/authentication/models"
	"ranchat/internals/authentication/repository"
	"ranchat/internals/authentication/utils"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserServices struct {
	Repo repository.UserRepository
	dis  *events.EventDispatcher
}

func NewUserService(repo repository.UserRepository, dispatcher *events.EventDispatcher) *UserServices {
	return &UserServices{Repo: repo, dis: dispatcher}
}

// AddUser creates a new user and returns a JWT token
func (u *UserServices) AddUser(ctx context.Context, user models.User) (string, dto.ErrorResponse, error) {
	if user.UserId.IsZero() {
		user.UserId = bson.NewObjectID()
	}

	token := utils.GenerateJWTToken(user)
	err := u.Repo.CreateUser(ctx, user)
	if err != nil {
		switch err {
		case errors.ErrDuplicateUser:
			return "", dto.ErrorResponse{StatusCode: 409, Message: "User already exists"}, err
		default:
			return "", dto.ErrorResponse{StatusCode: 500, Message: "Database error"}, err
		}
	}
	//publish the event
	u.dis.Publish(
		events.UserSignedUp,
		events.UserSignedUpEvent{
			UserID:    user.UserId.Hex(),
			DeviceID:  user.DeviceID,
			CreatedAt: time.Now(),
		},
	)
	return token, dto.ErrorResponse{}, nil
}

// UpdateUser applies partial updates to a user
func (u *UserServices) UpdateUser(ctx context.Context, updates map[string]any, id string) (models.User, dto.ErrorResponse, error) {
	objId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, dto.ErrorResponse{StatusCode: 500, Message: "Internal Server Error"}, err
	}
	updateObj := models.User{UserId: objId}

	if val, ok := updates["name"]; ok && val != nil {
		if name, ok := val.(string); ok {
			updateObj.Name = name
		}
	}
	if val, ok := updates["age"]; ok && val != nil {
		switch age := val.(type) {
		case int:
			updateObj.Age = age
		case float64:
			updateObj.Age = int(age)
		}
	}
	if val, ok := updates["bio"]; ok && val != nil {
		if bio, ok := val.(string); ok {
			updateObj.Bio = bio
		}
	}
	if val, ok := updates["interest"]; ok && val != nil {
		if interests, ok := val.([]string); ok {
			updateObj.Interest = interests
		}
	}

	updatedUser, err := u.Repo.UpdateUser(ctx, updateObj)
	if err != nil {
		switch err {
		case errors.ErrUserNotFound:
			return models.User{}, dto.ErrorResponse{StatusCode: 404, Message: "User not found"}, err
		default:
			return models.User{}, dto.ErrorResponse{StatusCode: 500, Message: "Database error"}, err
		}
	}
	return updatedUser, dto.ErrorResponse{}, nil
}

// FindUser retrieves a user by ID
func (u *UserServices) FindUser(ctx context.Context, id string) (models.User, dto.ErrorResponse, error) {
	user, err := u.Repo.GetUserById(ctx, id)
	if err != nil {
		switch err {
		case errors.ErrUserNotFound:
			return models.User{}, dto.ErrorResponse{StatusCode: 404, Message: "User not found"}, err
		default:
			return models.User{}, dto.ErrorResponse{StatusCode: 500, Message: "Database error"}, err
		}
	}
	return user, dto.ErrorResponse{}, nil
}

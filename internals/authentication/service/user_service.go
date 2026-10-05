package service

import (
	"context"
	"ranchat/errors"
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
func (u *UserServices) UpdateUser(
	ctx context.Context,
	updates map[string]any,
	id string,
) (models.User, dto.ErrorResponse, error) {

	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return models.User{},
			dto.ErrorResponse{
				StatusCode: 400,
				Message:    "Invalid user ID",
			},
			err
	}

	// Only allow fields that the user is allowed to update.
	allowedUpdates := make(map[string]any)

	if val, ok := updates["name"]; ok {
		name, ok := val.(string)
		if !ok {
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 400,
					Message:    "name must be a string",
				},
				nil
		}

		allowedUpdates["name"] = name
	}

	if val, ok := updates["age"]; ok {
		switch age := val.(type) {
		case float64:
			allowedUpdates["age"] = int(age)

		default:
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 400,
					Message:    "age must be a number",
				},
				nil
		}
	}
	if val, ok := updates["gender"]; ok {
		gender, ok := val.(string)
		if !ok {
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 400,
					Message:    "gender must be a string",
				},
				nil
		}

		switch gender {
		case string(models.GenderMale),
			string(models.GenderFemale),
			string(models.GenderOther):

			allowedUpdates["gender"] = gender

		default:
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 400,
					Message:    "invalid gender",
				},
				nil
		}
	}

	if val, ok := updates["bio"]; ok {
		bio, ok := val.(string)
		if !ok {
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 400,
					Message:    "bio must be a string",
				},
				nil
		}

		allowedUpdates["bio"] = bio
	}

	if val, ok := updates["interest"]; ok {
		interests, ok := val.([]any)
		if !ok {
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 400,
					Message:    "interest must be an array",
				},
				nil
		}

		interestList := make([]string, 0, len(interests))

		for _, item := range interests {
			interest, ok := item.(string)
			if !ok {
				return models.User{},
					dto.ErrorResponse{
						StatusCode: 400,
						Message:    "interest must contain strings",
					},
					nil
			}

			interestList = append(interestList, interest)
		}

		allowedUpdates["interest"] = interestList
	}

	if len(allowedUpdates) == 0 {
		return models.User{},
			dto.ErrorResponse{
				StatusCode: 400,
				Message:    "No valid fields provided for update",
			},
			nil
	}

	updatedUser, err := u.Repo.UpdateUser(
		ctx,
		objID,
		allowedUpdates,
	)

	if err != nil {
		switch err {
		case errors.ErrUserNotFound:
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 404,
					Message:    "User not found",
				},
				err

		default:
			return models.User{},
				dto.ErrorResponse{
					StatusCode: 500,
					Message:    "Database error",
				},
				err
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

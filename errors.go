package errors

import "errors"

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrDuplicateUser    = errors.New("duplicate user")
	ErrInvalidPayload   = errors.New("invalid payload")
	ErrDBFailure        = errors.New("database failure")
	ErrPresenceExists   = errors.New("presence already exists in memory")
	ErrPresenceNotFound = errors.New("presence not found in memory")
	ErrPresenceInactive = errors.New("presence is inactive")
)

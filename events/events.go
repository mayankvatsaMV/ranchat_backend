package events

import "time"

const UserSignedUp = "user.signedup"

type UserSignedUpEvent struct {
	UserID    string
	DeviceID  string
	CreatedAt time.Time
}

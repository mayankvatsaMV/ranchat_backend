package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

type User struct {
	UserId      bson.ObjectID `json:"user_id" bson:"_id,omitempty"`
	Name        string        `json:"name" bson:"name"`
	Age         int           `json:"age" bson:"age"`
	Gender      string        `json:"gender" bson:"gender"`
	Bio         string        `json:"bio" bson:"bio"`
	Interest    []string      `json:"interest" bson:"interest"`
	DeviceID    string        `bson:"deviceId"    json:"deviceId"`
	PremiumTill time.Time     `bson:"premiumTill" json:"premiumTill"` // nil = free user
	CreatedAt   time.Time     `bson:"createdAt"   json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updatedAt"   json:"updatedAt"`
}

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Friend struct {
	ID        bson.ObjectID   `bson:"_id,omitempty" json:"id,omitempty"`
	UserIDs   []bson.ObjectID `bson:"user_ids" json:"user_ids"`
	CreatedAt time.Time       `bson:"created_at" json:"created_at"`
}

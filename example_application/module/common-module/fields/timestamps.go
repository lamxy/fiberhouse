package fields

import (
	"time"
)

type Timestamps struct {
	CreatedAt time.Time `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at,omitempty"`
}

func NewTimestamps(now time.Time) Timestamps {
	now = now.UTC()
	return Timestamps{CreatedAt: now, UpdatedAt: now}
}

package model

import "time"

type User struct {
	Id        string    `bson:"_id,omitempty" json:"id"`
	Name      string    `bson:"name,omitempty" json:"name"`
	Username  string    `bson:"username,omitempty" json:"username"`
	Email     string    `bson:"email,omitempty" json:"email"`
	Password  string    `bson:"password,omitempty" json:"password"`
	CreatedAt time.Time `bson:"created_at,omitempty" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at,omitempty" json:"updated_at"`
}

package types

import "gorm.io/gorm"

type APIMessage struct {
	Content string `json:"content"`
	PubDate string `json:"pub_date"`
	User    string `json:"username"`
}

type User struct {
	gorm.Model
	Username string `gorm:"size:100;not null;unique" json:"username"`
	Email    string `gorm:"size:100;not null" json:"email"`
	PWHash   string `gorm:"size:255;not null" json:"pw_hash"`
}

type UserDetails struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Follower struct {
	Who  User `gorm:"foreignKey:WhoID;constraint:OnDelete:CASCADE"`
	Whom User `gorm:"foreignKey:WhomID;constraint:OnDelete:CASCADE"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

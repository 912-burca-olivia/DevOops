package types

import "gorm.io/gorm"

// APIMessage represents a message in API responses (not a GORM model).
type APIMessage struct {
	Content string `json:"content"`
	PubDate int64  `json:"pub_date"`
	User    string `json:"user"`
}

// Message is the GORM model for a tweet/message.
type Message struct {
	AuthorID uint   `gorm:"not null;index" json:"author_id"`
	Text     string `gorm:"not null" json:"text"`
	PubDate  int64  `gorm:"index" json:"pub_date"`
	Flagged  int    `gorm:"index" json:"flagged"`
}

// User is the GORM model for a registered user.
type User struct {
	gorm.Model
	Username string `gorm:"size:100;not null;unique" json:"username"`
	Email    string `gorm:"size:100;not null;unique" json:"email"`
	PWHash   string `gorm:"size:255;not null" json:"pw_hash"`
}

// Follower represents a "follow" relationship between two users.
type Follower struct {
	gorm.Model
	WhoID  uint `gorm:"not null;index;uniqueIndex:idx_follow,priority:1" json:"who_id"`  // follower (who follows someone)
	WhomID uint `gorm:"not null;index;uniqueIndex:idx_follow,priority:2" json:"whom_id"` // followee (who is followed)

	Who  User `gorm:"foreignKey:WhoID;constraint:OnDelete:CASCADE"`
	Whom User `gorm:"foreignKey:WhomID;constraint:OnDelete:CASCADE"`
}

// LoginRequest represents the expected JSON body for login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

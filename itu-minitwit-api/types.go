package main

type APIMessage struct {
	Content string `json:"content"`
	PubDate string `json:"pub_date"`
	User    string `json:"username"`
}

type UserDetails struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type User struct {
	UserID    uint       `gorm:"column:user_id;primaryKey"`
	Username  string     `gorm:"unique;not null" json:"username"`
	Email     string     `gorm:"not null" json:"email"`
	PWHash    string     `gorm:"not null" json:"pw_hash"`
	Messages  []Message  `gorm:"foreignKey:AuthorID"` // One-to-Many with Message
	Followers []Follower `gorm:"foreignKey:WhomID"`   // Followers list
	Following []Follower `gorm:"foreignKey:WhoID"`    // Following list
}

type Message struct {
	MessageID uint   `gorm:"primaryKey"`
	AuthorID  uint   `gorm:"not null"`
	Author    User   `gorm:"foreignKey:AuthorID;references:UserID"`
	Text      string `gorm:"not null"`
	PubDate   string `gorm:"not null"`
	Flagged   uint   `gorm:"default:0"`
}

type Follower struct {
	WhoID  uint `gorm:"not null"`
	Who    User `gorm:"foreignKey:WhoID;references:UserID"`
	WhomID uint `gorm:"not null"`
	Whom   User `gorm:"foreignKey:WhomID;references:UserID"`
}

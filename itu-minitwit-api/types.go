package main

type APIMessage struct {
	Content string `json:"test"`
	PubDate string `json:"pub_date"`
	User 	string `json:"username"` 
}

type User struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	PWHash   string `json:"pw_hash"`
}

type UserDetails struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Follower struct {
	WhoID  int `json:"who_id"`
	WhomID int `json:"whom_id"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
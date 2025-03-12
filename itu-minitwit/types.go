package main

type Message struct {
	Text     string `json:"content"`
	PubDate  string `json:"pub_date"`
	Username string `json:"user"`
}

type UserDetails struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

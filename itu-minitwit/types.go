package main

type Message struct {
	MessageID int    `json:"message_id"`
	AuthorID  int    `json:"author_id"`
	Text      string `json:"text"`
	PubDate   int    `json:"pub_date"`
	Flagged   int    `json:"flagged"`
	Username  string `json:"username"`
	Email     string `json:"email"`
}

type User struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	PWHash   string `json:"pw_hash"`
}

type follower struct {
	WhoID  int `json:"who_id"`
	WhomID int `json:"whom_id"`
}

type TemplateMessage struct {
	Message
	Email string
}

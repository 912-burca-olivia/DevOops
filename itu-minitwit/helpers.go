package main

import (
	"database/sql"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		log.Println(err)
	}
	return string(bytes)
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func Error() string {
	return "An error occurred."
}

// getUserID retrieves the user_id for a given username.
func getUserID(db *sql.DB, username string) (int, error) {
	var userID int
	err := db.QueryRow("SELECT user_id FROM user WHERE username = ?", username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // Return -1 if no user is found
		}
		return -999, err // Return the error if there's another issue
	}
	return userID, nil
}

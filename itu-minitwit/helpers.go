package main

import (
	"database/sql"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// Taken from https://gowebexamples.com/password-hashing/

func HashPassword(password string) (string) {
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

// getUserID retrieves the user_id for a given username.
func getUserID(db *sql.DB, username string) (int,error) {
	var userID int
	err := db.QueryRow("SELECT user_id FROM user WHERE username = ?", username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1,nil // Return -1 if no user is found
		}
		return -999,err // Return the error
	}
	return userID, nil // Return userID if the user exists
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

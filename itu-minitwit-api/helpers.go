package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"minitwit/types"
)

// Taken from https://gowebexamples.com/password-hashing/

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
		return -999, err // Return the error
	}
	return userID, nil // Return userID if the user exists
}

// InitDB initializes the database using GORM migrations
func InitDB() {
	// Connect to the database
	ConnectDB()

	// Run migrations to ensure tables exist
	err := DB.AutoMigrate(&types.User{}, &types.Follower{})
	if err != nil {
		log.Fatalf("❌ Database migration failed: %v", err)
	}

	fmt.Println("✅ Database initialized successfully using GORM")
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func NotReqFromSimulator(w http.ResponseWriter, r *http.Request) bool {
	fromSimulator := r.Header.Get("Authorization")
	if fromSimulator != "Basic c2ltdWxhdG9yOnN1cGVyX3NhZmUh" {
		w.WriteHeader(http.StatusForbidden)
		response := map[string]interface{}{
			"status":    http.StatusForbidden,
			"error_msg": "You are not authorized to use this resource!",
		}
		json.NewEncoder(w).Encode(response)
		return true
	}
	return false
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"gorm.io/gorm"
)

// Taken from https://gowebexamples.com/password-hashing/

func Error() string {
	return "An error occurred."
}

// getUserID retrieves the user_id for a given username.
// func getUserID(db *gorm.DB, username string) (int, error) {
// 	/* TODO - use orm instead of query
// 	var userID int
// 	err := db.QueryRow("SELECT user_id FROM user WHERE username = ?", username).Scan(&userID)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return -1, nil // Return -1 if no user is found
// 		}
// 		return -999, err // Return the error
// 	}
// 	return userID, nil // Return userID if the user exists
// 	*/
// 	return -1, nil // remove this when the method is done
// }

func getUserID(db *gorm.DB, username string) (uint, error) {
	var user User
	result := db.Select("user_id").Where("username = ?", username).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return 0, nil // user not found
		}
		return 0, result.Error
	}
	return user.UserID, nil
}

func initDB() {
	// Open database connection
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// Get the underlying SQL database object to close the db
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get DB: %v", err)
	}
	defer sqlDB.Close()

	// Auto-migrate the schema
	err = db.AutoMigrate(&User{}, &Follower{}, &Message{})
	if err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	fmt.Println("Database initialized successfully")
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

// RespondJSONError sends a JSON error response with a given status code and message.
func RespondJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    statusCode,
		"error_msg": message,
	})
}

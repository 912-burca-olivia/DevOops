package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	//"os"
	"gorm.io/gorm"
)

// Taken from https://gowebexamples.com/password-hashing/

func Error() string {
	return "An error occurred."
}

// getUserID retrieves the user_id for a given username.
func (api API) getUserID(db *gorm.DB, username string) (uint, error) {
	var user User
	result := db.Select("user_id").Where("username = ?", username).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			api.metrics.UserNotFound.WithLabelValues("Users_not_found").Inc()
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
	// sqlDB, err := db.DB()
	// if err != nil {
	// 	log.Fatalf("Failed to get DB: %v", err)
	// }
	// defer sqlDB.Close()

	// Auto-migrate the schema
	err = db.AutoMigrate(&User{}, &Follower{}, &Message{})
	if err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	fmt.Println("Database initialized successfully")
}

// func fileExists(filename string) bool {
// 	info, err := os.Stat(filename)
// 	if os.IsNotExist(err) {
// 		return false
// 	}
// 	return !info.IsDir()
// }

func NotReqFromSimulator(w http.ResponseWriter, r *http.Request) bool {
	fromSimulator := r.Header.Get("Authorization")
	if fromSimulator != "Basic c2ltdWxhdG9yOnN1cGVyX3NhZmUh" {
		w.WriteHeader(http.StatusForbidden)
		response := map[string]interface{}{
			"status":    http.StatusForbidden,
			"error_msg": "You are not authorized to use this resource!",
		}
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			fmt.Print(err.Error())
			return false
		}
		return true
	}
	return false
}

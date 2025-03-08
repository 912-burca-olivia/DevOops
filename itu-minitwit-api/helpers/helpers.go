package helpers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"minitwit/types"
)

const DATABASE = "minitwit.db"
const PER_PAGE = 30

var (
	DB   *gorm.DB
	once sync.Once
)

// ConnectDB initializes a SQLite database connection (thread-safe singleton).
func ConnectDB() {
	once.Do(func() {
		dbPath := os.Getenv("DATABASE")
		if dbPath == "" {
			dbPath = DATABASE
		}
		var err error
		DB, err = gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		fmt.Println("Connected to SQLite database:", dbPath)
	})
}

// InitDB loads the schema and performs auto-migration for GORM models.
func InitDB() {
	ConnectDB()

	// Load schema.sql and execute it to set up tables
	schemaSQL, err := os.ReadFile("schema.sql")
	if err != nil {
		log.Fatalf("Failed to read schema.sql: %v", err)
	}
	if err := DB.Exec(string(schemaSQL)).Error; err != nil {
		log.Fatalf("Failed to execute schema.sql: %v", err)
	}

	// Run GORM auto-migrations to apply any model constraints (indexes, etc.)
	if err := DB.AutoMigrate(&types.User{}, &types.Follower{}, &types.Message{}); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	fmt.Println("Database initialized successfully using GORM")
}

// GetUserID retrieves the user ID for a given username. Returns 0 if not found.
func GetUserID(username string) (uint, error) {
	var user types.User
	result := DB.Select("id").Where("username = ?", username).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return 0, nil // user not found
		}
		return 0, result.Error
	}
	return user.ID, nil
}

// NotReqFromSimulator verifies the request's Authorization header for simulator access.
func NotReqFromSimulator(w http.ResponseWriter, r *http.Request) bool {
	expectedAuth := "Basic c2ltdWxhdG9yOnN1cGVyX3NhZmUh"
	if r.Header.Get("Authorization") != expectedAuth {
		RespondJSONError(w, http.StatusForbidden, "You are not authorized to use this resource!")
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

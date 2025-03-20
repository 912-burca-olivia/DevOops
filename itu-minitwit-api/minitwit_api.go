package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const DATABASE = "../minitwit.db"
const PER_PAGE = 30

var db *gorm.DB
var err error

var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

func connectDB() (*gorm.DB, error) {

	host := os.Getenv("DB_HOST")
	if host == "" { // sqlite locally - for now
		db, err = gorm.Open(sqlite.Open(DATABASE), &gorm.Config{})
	} else { // postgresql remote
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
		)
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		return nil, err
	}

	return db, nil
}

func FormatDateTime(timestamp int64) string {
	t := time.Unix(timestamp, 0)
	return t.Format("Jan 2, 2006 at 3:04PM")
}

func UpdateLatest(r *http.Request) {
	parsedCommandID := -1
	if latestParam := r.URL.Query().Get("latest"); latestParam != "" {
		if id, err := strconv.Atoi(latestParam); err == nil {
			parsedCommandID = id
		}
	}

	if parsedCommandID != -1 {
		file, err := os.Create("./latest_processed_sim_action_id.txt")
		if err == nil {
			defer file.Close()
			file.WriteString(strconv.Itoa(parsedCommandID))
		}
	}
}

func GETLatestHandler(w http.ResponseWriter, r *http.Request) {
	// Read the latest processed action ID from a file
	UpdateLatest(r)
	content, err := os.ReadFile("latest_processed_sim_action_id.txt")
	if err != nil {
		http.Error(w, "Failed to read latest action ID", http.StatusInternalServerError)
		return
	}

	latestID := strings.TrimSpace(string(content))
	if latestID == "" {
		latestID = "-1"
	}

	w.Header().Set("Content-Type", "application/json")
	latestID_int, _ := strconv.Atoi(latestID)
	json.NewEncoder(w).Encode(map[string]int{"latest": latestID_int})
}

func GetNumberHandler(r *http.Request) int {
	parsedCommandID := 100
	if number := r.URL.Query().Get("no"); number != "" {
		if id, err := strconv.Atoi(number); err == nil {
			parsedCommandID = id
		}
	}
	return parsedCommandID
}

func GETFollowerHandler(w http.ResponseWriter, r *http.Request) {

	//number of requested followers
	rowNums := GetNumberHandler(r)

	UpdateLatest(r)
	vars := mux.Vars(r)

	userID, _ := getUserID(db, vars["username"])

	if userID == 0 {
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	// Query all followers

	var followers []string

	err = db.
		Table("users").
		Select("users.username").
		Joins("INNER JOIN followers ON followers.whom_id = users.user_id").
		Where("followers.who_id = ?", userID).
		Limit(rowNums).
		Pluck("username", &followers).
		Error

	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	response := map[string][]string{"follows": followers}
	json.NewEncoder(w).Encode(response)

}

func POSTFollowerHandler(w http.ResponseWriter, r *http.Request) {

	UpdateLatest(r)

	vars := mux.Vars(r)

	userID, _ := getUserID(db, vars["username"])

	if userID == 0 {
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	var data map[string]interface{}

	json.NewDecoder(r.Body).Decode(&data)

	if followsUsername, exists := data["follow"]; exists {
		followsUserID, _ := getUserID(db, followsUsername.(string))
		if followsUserID == 0 {
			http.Error(w, "The user you are trying to follow cannot be found", http.StatusNotFound)
			return
		}

		// Insert follow relationship
		follower := Follower{WhoID: userID, WhomID: followsUserID}

		err := db.Create(&follower).Error

		if err != nil {
			http.Error(w, "Failed to follow user", http.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(data)
		return
	} else if unfollowsUsername, exists := data["unfollow"]; exists {
		unfollowsUserID, _ := getUserID(db, unfollowsUsername.(string))
		if unfollowsUserID == 0 {
			http.Error(w, "The user you are trying to unfollow cannot be found", http.StatusNotFound)
			return
		}
		// Delete follow relationship
		err := db.Where("who_id = ? AND whom_id = ?", userID, unfollowsUserID).Delete(&Follower{}).Error

		if err != nil {
			http.Error(w, "Failed to unfollow user", http.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(data)
		return
	}

}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {

	UpdateLatest(r) // Updater the latest parameter

	var error = ""

	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)

	username := data["username"].(string)
	email := data["email"].(string)
	password := data["pwd"].(string)
	// Validate input fields
	if username == "" {
		error = "You have to enter a username"
	} else if email == "" {
		error = "You have to enter a valid email address"
	} else if !strings.Contains(email, "@") {
		error = "You have to enter a valid email address"
	} else if password == "" {
		error = "You have to enter a password"
	} else {
		// Check if the username is already taken
		userId, _ := getUserID(db, username)

		if userId != 0 {
			error = "The username is already taken"
		} else {
			// Insert new user into the database
			newUser := User{Username: username, Email: email, PWHash: password}
			err := db.Create(&newUser).Error
			if err != nil {
				log.Println("Error inserting user:", err)
				http.Error(w, "Failed to register user", http.StatusInternalServerError)
				return
			}
		}
	}
	var status int

	if error == "" {
		w.WriteHeader(http.StatusNoContent)
		status = 200
	} else {
		w.WriteHeader(http.StatusBadRequest)
		status = 400
	}
	response := map[string]interface{}{
		"status":    status,
		"error_msg": error,
	}
	json.NewEncoder(w).Encode(response)
}

func GETAllMessagesHandler(w http.ResponseWriter, r *http.Request) {

	UpdateLatest(r)

	// Retrieve all non-flagged messages
	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = 0").
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error

	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(messages) == 0 {
		w.Write([]byte("[]"))
		return
	}

	var filteredMsgs []map[string]string

	for _, msg := range messages {
		filteredMsg := map[string]string{
			"content":  msg.Content,
			"pub_date": msg.PubDate,
			"user":     msg.User,
		}
		filteredMsgs = append(filteredMsgs, filteredMsg)
	}

	json.NewEncoder(w).Encode(filteredMsgs)
}

func GETUserMessagesHandler(w http.ResponseWriter, r *http.Request) {

	UpdateLatest(r)

	username := mux.Vars(r)["username"]

	// Get user ID
	userID, err := getUserID(db, username)
	if err != nil || userID == 0 {
		fmt.Printf("Cannot find user: %s", username)
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	// Retrieve messages
	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = 0 AND users.user_id = ?", userID).
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error

	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	// Ensure empty response is always a valid JSON array
	w.Header().Set("Content-Type", "application/json")
	if len(messages) == 0 {
		w.Write([]byte("[]"))
		return
	}

	var filteredMsgs []map[string]string

	for _, msg := range messages {
		filteredMsg := map[string]string{
			"content":  msg.Content,
			"pub_date": msg.PubDate,
			"user":     msg.User,
		}
		filteredMsgs = append(filteredMsgs, filteredMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredMsgs)
}

func POSTMessagesHandler(w http.ResponseWriter, r *http.Request) {

	UpdateLatest(r)

	username := mux.Vars(r)["username"]

	// Get user ID
	userID, err := getUserID(db, username)
	if err != nil || userID == 0 {
		fmt.Printf("Cannot find user: %s", username)
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	// Read and decode request body
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil || strings.TrimSpace(data.Content) == "" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"status":400, "error_msg":"Invalid or missing content"}`, http.StatusBadRequest)
		return
	}

	// Create and save message
	message := Message{
		AuthorID: userID,
		Text:     data.Content,
		PubDate:  FormatDateTime(time.Now().Unix()),
		Flagged:  0,
	}

	// Insert into DB
	if err := db.Create(&message).Error; err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": http.StatusBadRequest,
			"res":    err.Error(),
		})
		return
	}

	// Successful response
	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": http.StatusNoContent,
		"res":    "",
	})
}

func GETUserDetailsHandler(w http.ResponseWriter, r *http.Request) {

	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")

	var user User
	if userID != "" {
		result := db.Where("user_id = ?", userID).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error: "+result.Error.Error(), http.StatusInternalServerError)
			}
			return
		}
	} else if username != "" {
		result := db.Where("username = ?", username).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error: "+result.Error.Error(), http.StatusInternalServerError)
			}
			return
		}
	} else {
		// If neither user_id nor username is provided, return an error
		http.Error(w, "Missing user_id or username query parameter", http.StatusBadRequest)
		return
	}

	userDetails := UserDetails{
		UserID:   user.UserID,
		Username: user.Username,
		Email:    user.Email,
	}

	json.NewEncoder(w).Encode(userDetails)
}

func GETFollowingHandler(w http.ResponseWriter, r *http.Request) {

	whoUsername := r.URL.Query().Get("whoUsername")
	whomUsername := r.URL.Query().Get("whomUsername")
	whoUsernameID, _ := getUserID(db, whoUsername)
	whomUsernameID, _ := getUserID(db, whomUsername)

	var isFollowing bool = true
	var follower Follower
	err = db.Model(&Follower{}).
		Where("who_id = ? AND whom_id = ?", whoUsernameID, whomUsernameID).
		First(&follower).Error

	if err != nil {
		isFollowing = false // Default to false if no rows found or any error occurs
	}

	json.NewEncoder(w).Encode(isFollowing)

}

func PostLoginHandler(w http.ResponseWriter, r *http.Request) {

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if user exists
	var foundUser User
	result := db.Debug().Where("username = ?", req.Username).First(&foundUser) //db.Where("username = ?", req.Username).First(&foundUser)
	if result.Error == gorm.ErrRecordNotFound {
		http.Error(w, "Invalid credentials", http.StatusNotFound)
		return
	} else if result.Error != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// At this point we know that a user exists
	// Check the password hash against the one found in the db
	if req.Password == foundUser.PWHash {
		w.WriteHeader(http.StatusOK)
	} else {

		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
}

func GetFollowingMessages(w http.ResponseWriter, r *http.Request) {

	var userID = r.URL.Query().Get("userid")

	var messages []APIMessage

	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON users.user_id = messages.author_id").
		Where("flagged = ? AND (author_id = ? OR author_id IN (SELECT who_id FROM followers WHERE whom_id = ?))", 0, userID, userID).
		Order("messages.pub_date DESC").
		Limit(PER_PAGE).
		Find(&messages).Error

	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	// Convert to Json app Message format

	var filteredMsgs []map[string]string

	for _, msg := range messages {
		filteredMsg := map[string]string{
			"content":  msg.Content,
			"pub_date": msg.PubDate,
			"user":     msg.User,
		}
		filteredMsgs = append(filteredMsgs, filteredMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredMsgs)

}

func main() {
	// Create a new mux router
	initDB()
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 16, // 16 hours
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	r := mux.NewRouter()

	// Define the routes and their handlers
	r.HandleFunc("/latest", GETLatestHandler).Methods("GET")
	r.HandleFunc("/register", RegisterHandler).Methods("POST")
	r.HandleFunc("/fllws/{username}", POSTFollowerHandler).Methods("POST")
	r.HandleFunc("/fllws/{username}", GETFollowerHandler).Methods("GET")
	r.HandleFunc("/msgs", GETAllMessagesHandler).Methods("GET")
	r.HandleFunc("/msgs/{username}", GETUserMessagesHandler).Methods("GET")
	r.HandleFunc("/msgs/{username}", POSTMessagesHandler).Methods("POST")
	r.HandleFunc("/followingmsgs", GetFollowingMessages).Methods("GET")
	r.HandleFunc("/getUserDetails", GETUserDetailsHandler).Methods("GET")
	r.HandleFunc("/isfollowing", GETFollowingHandler).Methods("GET")
	r.HandleFunc("/login", PostLoginHandler).Methods("POST")
	// Start the server on port 9090
	fmt.Println("Server starting on http://localhost:9090")
	log.Fatal(http.ListenAndServe(":9090", r))
}

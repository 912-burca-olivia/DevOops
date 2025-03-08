package main

import (
	"encoding/json"
	"fmt"
	"log"
	"minitwit/helpers"
	"minitwit/types"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
)

var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

// UpdateLatest writes the provided "latest" query parameter (if any) to a file for simulator synchronization.
func UpdateLatest(r *http.Request) {
	latestParam := r.URL.Query().Get("latest")
	if latestParam == "" {
		return
	}
	if id, err := strconv.Atoi(latestParam); err == nil {
		// Record the latest processed action ID
		if file, err := os.Create("latest_processed_sim_action_id.txt"); err == nil {
			defer file.Close()
			file.WriteString(strconv.Itoa(id))
		}
	}
}

func GETLatestHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	content, err := os.ReadFile("latest_processed_sim_action_id.txt")
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to read latest action ID")
		return
	}
	latestID := strings.TrimSpace(string(content))
	if latestID == "" {
		latestID = "-1"
	}
	latestVal, _ := strconv.Atoi(latestID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"latest": latestVal})
}

func GetNumberHandler(r *http.Request) int {
	// Utility to fetch "no" (number of items) parameter, default to 100 if not provided
	count := 100
	if numStr := r.URL.Query().Get("no"); numStr != "" {
		if n, err := strconv.Atoi(numStr); err == nil {
			count = n
		}
	}
	return count
}

func GETFollowerHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	vars := mux.Vars(r)

	// Get user by username
	var user types.User
	if err := helpers.DB.Where("username = ?", vars["username"]).First(&user).Error; err != nil {
		helpers.RespondJSONError(w, http.StatusNotFound, "Cannot find user")
		return
	}

	// Retrieve usernames of people this user follows
	var followerUsernames []string
	err := helpers.DB.Table("users").
		Joins("JOIN followers ON followers.whom_id = users.id").
		Where("followers.who_id = ?", user.ID).
		Limit(GetNumberHandler(r)).
		Pluck("users.username", &followerUsernames).Error
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to retrieve follows list")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"follows": followerUsernames})
}

func POSTFollowerHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	vars := mux.Vars(r)

	// Get user who is performing the action
	var user types.User
	if err := helpers.DB.Where("username = ?", vars["username"]).First(&user).Error; err != nil {
		helpers.RespondJSONError(w, http.StatusNotFound, "Cannot find user")
		return
	}

	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Handle follow action
	if followsUsername, exists := data["follow"]; exists {
		var followsUser types.User
		if err := helpers.DB.Where("username = ?", followsUsername).First(&followsUser).Error; err != nil {
			helpers.RespondJSONError(w, http.StatusNotFound, "The user you are trying to follow cannot be found")
			return
		}
		if user.ID == followsUser.ID {
			helpers.RespondJSONError(w, http.StatusBadRequest, "You cannot follow yourself")
			return
		}
		follow := types.Follower{WhoID: user.ID, WhomID: followsUser.ID}
		if err := helpers.DB.Create(&follow).Error; err != nil {
			// Could be a duplicate follow or DB error
			helpers.RespondJSONError(w, http.StatusBadRequest, "Could not follow user")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// Handle unfollow action
	if unfollowsUsername, exists := data["unfollow"]; exists {
		var unfollowsUser types.User
		if err := helpers.DB.Where("username = ?", unfollowsUsername).First(&unfollowsUser).Error; err != nil {
			helpers.RespondJSONError(w, http.StatusNotFound, "The user you are trying to unfollow cannot be found")
			return
		}
		if err := helpers.DB.Where("who_id = ? AND whom_id = ?", user.ID, unfollowsUser.ID).Delete(&types.Follower{}).Error; err != nil {
			helpers.RespondJSONError(w, http.StatusBadRequest, "Could not unfollow user")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// No valid follow/unfollow key provided
	helpers.RespondJSONError(w, http.StatusBadRequest, "Missing follow or unfollow command")
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)

	var data map[string]string
	json.NewDecoder(r.Body).Decode(&data)

	username := strings.TrimSpace(data["username"])
	email := strings.TrimSpace(data["email"])
	pwd := data["pwd"]

	// Input validation
	if username == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "You have to enter a username")
		return
	}
	if email == "" || !strings.Contains(email, "@") {
		helpers.RespondJSONError(w, http.StatusBadRequest, "You have to enter a valid email address")
		return
	}
	if pwd == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "You have to enter a password")
		return
	}

	// Check if username is already taken
	var existing types.User
	err := helpers.DB.Where("username = ?", username).First(&existing).Error
	if err == nil {
		// Found a user with the same username
		helpers.RespondJSONError(w, http.StatusBadRequest, "The username is already taken")
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		// Unexpected database error
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to check existing user")
		return
	}

	// Create new user with hashed password
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}
	newUser := types.User{
		Username: username,
		Email:    email,
		PWHash:   string(hash),
	}
	if err := helpers.DB.Create(&newUser).Error; err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	// Successfully created, no content to return
	w.WriteHeader(http.StatusNoContent)
}

func GETAllMessagesHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)

	var messages []types.APIMessage
	err := helpers.DB.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.id").
		Where("messages.flagged = 0").
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to retrieve messages")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func GETUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Missing username parameter")
		return
	}

	// Fetch the user ID (to ensure user exists)
	userID, err := helpers.GetUserID(username)
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Error retrieving user")
		return
	}
	if userID == 0 {
		helpers.RespondJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	// Retrieve this user's messages without an extra join (we already know the username)
	var msgs []types.Message
	err = helpers.DB.Where("flagged = 0 AND author_id = ?", userID).
		Order("pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&msgs).Error
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to retrieve messages")
		return
	}
	// Transform to APIMessage output
	apiMessages := make([]types.APIMessage, 0, len(msgs))
	for _, m := range msgs {
		apiMessages = append(apiMessages, types.APIMessage{
			Content: m.Text,
			PubDate: m.PubDate,
			User:    username,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiMessages)
}

func POSTMessagesHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Missing username parameter")
		return
	}

	// Get user ID for the author of the message
	userID, err := helpers.GetUserID(username)
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Error retrieving user")
		return
	}
	if userID == 0 {
		helpers.RespondJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	content, ok := data["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Content is required")
		return
	}

	message := types.Message{
		AuthorID: userID,
		Text:     content,
		PubDate:  time.Now().Unix(),
		Flagged:  0,
	}
	if err := helpers.DB.Create(&message).Error; err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to create message")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": http.StatusNoContent,
		"res":    "Message posted successfully",
	})
}

func GETUserDetailsHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	userIDStr := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")

	if userIDStr == "" && username == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Either user_id or username is required")
		return
	}

	var user types.User
	var err error
	if userIDStr != "" {
		userID, convErr := strconv.Atoi(userIDStr)
		if convErr != nil {
			helpers.RespondJSONError(w, http.StatusBadRequest, "Invalid user_id format")
			return
		}
		err = helpers.DB.Where("id = ?", userID).First(&user).Error
	} else {
		err = helpers.DB.Where("username = ?", username).First(&user).Error
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			helpers.RespondJSONError(w, http.StatusNotFound, "User not found")
		} else {
			helpers.RespondJSONError(w, http.StatusInternalServerError, "Database query failed")
		}
		return
	}

	var userDetails struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userDetails)
}

func GETFollowingHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	whoUsername := r.URL.Query().Get("whoUsername")
	whomUsername := r.URL.Query().Get("whomUsername")

	if whoUsername == "" || whomUsername == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Both whoUsername and whomUsername are required")
		return
	}

	whoUserID, err := helpers.GetUserID(whoUsername)
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Error retrieving whoUsername")
		return
	}
	if whoUserID == 0 {
		helpers.RespondJSONError(w, http.StatusNotFound, "whoUsername not found")
		return
	}
	whomUserID, err := helpers.GetUserID(whomUsername)
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Error retrieving whomUsername")
		return
	}
	if whomUserID == 0 {
		helpers.RespondJSONError(w, http.StatusNotFound, "whomUsername not found")
		return
	}

	var followers types.Follower
	err = helpers.DB.Where("who_id = ? AND whom_id = ?", whoUserID, whomUserID).First(&followers).Error
	var isFollowing bool
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			helpers.RespondJSONError(w, http.StatusInternalServerError, "Database query failed")
			return
		}
		isFollowing = false
	} else {
		isFollowing = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"is_following": isFollowing})
}

func GetFollowingMessages(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	userIDStr := r.URL.Query().Get("userid")
	if userIDStr == "" {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Missing userid parameter")
		return
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Invalid userid format")
		return
	}

	// Check if the user exists
	var user types.User
	if err := helpers.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			helpers.RespondJSONError(w, http.StatusNotFound, "User not found")
		} else {
			helpers.RespondJSONError(w, http.StatusInternalServerError, "Database query failed")
		}
		return
	}

	// Retrieve messages for the user and those they follow
	var messages []types.APIMessage
	err = helpers.DB.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.id").
		Where(`messages.flagged = 0 AND (
               messages.author_id = ? 
               OR messages.author_id IN (SELECT who_id FROM followers WHERE whom_id = ?))`,
			userID, userID).
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error
	if err != nil {
		helpers.RespondJSONError(w, http.StatusInternalServerError, "Failed to fetch messages")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func PostLoginHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)
	var req types.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helpers.RespondJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Find the user by username
	var user types.User
	if err := helpers.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			helpers.RespondJSONError(w, http.StatusUnauthorized, "Invalid credentials")
		} else {
			helpers.RespondJSONError(w, http.StatusInternalServerError, "Database query failed")
		}
		return
	}

	// Compare the provided password with the stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PWHash), []byte(req.Password)); err != nil {
		helpers.RespondJSONError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Login successful"})
}

func main() {
	helpers.InitDB()

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 16, // 16 hours
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	r := mux.NewRouter()

	// Define API routes and handlers
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

	fmt.Println("Server starting on http://localhost:9090")
	log.Fatal(http.ListenAndServe(":9090", r))
}

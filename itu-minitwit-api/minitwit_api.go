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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const DATABASE = "../minitwit.db"
const PER_PAGE = 30

var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

//const PER_PAGE = 30 //useful for the html template but not for the API implementation

func connectDB() (*gorm.DB, error) {
	databasePath := os.Getenv("DATABASE")
	if databasePath == "" {
		databasePath = DATABASE // Fallback in case the env variable is missing
	}

	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
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
	/* TODO - use orm instead of query

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	//number of requested followers
	rowNums := GetNumberHandler(r)

	UpdateLatest(r)
	vars := mux.Vars(r)

	userID, _ := getUserID(db, vars["username"])

	if userID == -1 {
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	// Query all followers
	query := `
				SELECT user.username FROM user
				INNER JOIN follower ON follower.whom_id=user.user_id
				WHERE follower.who_id=?
				LIMIT ?
			`

	rows, err := db.Query(query, userID, rowNums)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect followers
	var followers []string
	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		followers = append(followers, follower)
	}

	response := map[string][]string{"follows": followers}
	json.NewEncoder(w).Encode(response)
	*/
}

func POSTFollowerHandler(w http.ResponseWriter, r *http.Request) {

	UpdateLatest(r)

	db, err := connectDB()
	db.Debug()

	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

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
	db, err := connectDB()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "500",
			"error_msg": "Database connection failed",
		})
		return
	}

	UpdateLatest(r)
	w.Header().Set("Content-Type", "application/json")

	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "400",
			"error_msg": "Empty request body",
		})
		return
	}
	defer r.Body.Close()

	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "400",
			"error_msg": "Invalid JSON format",
		})
		return
	}

	// Extract and validate required fields
	username, email, pwd := strings.TrimSpace(data["username"]), strings.TrimSpace(data["email"]), data["pwd"]

	if username == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "400",
			"error_msg": "You have to enter a username",
		})
		return
	}
	if email == "" || !strings.Contains(email, "@") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "400",
			"error_msg": "You have to enter a valid email address",
		})
		return
	}
	if pwd == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "400",
			"error_msg": "You have to enter a password",
		})
		return
	}

	// Check if the username already exists
	var existing User
	if err := db.Where("username = ?", username).First(&existing).Error; err == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "400",
			"error_msg": "The username is already taken",
		})
		return
	} else if err != gorm.ErrRecordNotFound {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "500",
			"error_msg": "Database error while checking username",
		})
		return
	}

	// Create and save new user
	newUser := User{Username: username, Email: email, PWHash: pwd}
	if err := db.Create(&newUser).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "500",
			"error_msg": "Failed to register user",
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func GETAllMessagesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_msg": "Database connection failed"})
		return
	}
	UpdateLatest(r)

	// Retrieve all non-flagged messages
	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = false").
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_msg": "Failed to retrieve messages"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(messages) == 0 {
		w.Write([]byte("[]"))
		return
	}

	json.NewEncoder(w).Encode(messages)
}

func GETUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_msg": "Database connection failed"})
		return
	}
	UpdateLatest(r)

	username := mux.Vars(r)["username"]

	// Retrieve messages
	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = false AND users.username = ?", username).
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_msg": "Failed to retrieve messages"})
		return
	}

	// Ensure empty response is always a valid JSON array
	w.Header().Set("Content-Type", "application/json")
	if len(messages) == 0 {
		w.Write([]byte("[]"))
		return
	}

	json.NewEncoder(w).Encode(messages)
}

func POSTMessagesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"status":500, "error_msg":"Database connection failed"}`, http.StatusInternalServerError)
		return
	}
	UpdateLatest(r)

	username := mux.Vars(r)["username"]

	// Get user ID
	userID, err := getUserID(db, username)
	if err != nil || userID == 0 {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"status":404, "error_msg":"User not found"}`, http.StatusNotFound)
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
		Flagged:  false,
	}
	if err := db.Create(&message).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"status":500, "error_msg":"Failed to create message"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func GETUserDetailsHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

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
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	whoUsername := r.URL.Query().Get("whoUsername")
	whomUsername := r.URL.Query().Get("whomUsername")
	whoUsernameID, _ := getUserID(db, whoUsername)
	whomUsernameID, _ := getUserID(db, whomUsername)

	var isFollowing bool
	result := db.Select("whoUsername = ?", whoUsernameID).Where("whomUsername = ?", whomUsernameID).First(&isFollowing)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "User is not following", http.StatusNotFound)
			return //what do we return here? used to be only fmt.Println("User is not following")
		} else {
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
		}
	}
	/*err = db.QueryRow(
	      `select 1
	      from follower
	      where follower.who_id = ? and follower.whom_id = ?`,
	      whoUsernameID,
	      whomUsernameID).
	      Scan(&isFollowing)
	  if err != nil {
	      if err == sql.ErrNoRows {
	          fmt.Println("User is not following")
	      } else {
	          http.Error(w, "Database connection failed", http.StatusInternalServerError)
	      }
	  }*/
	json.NewEncoder(w).Encode(isFollowing)

}

func PostLoginHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if user exists
	var foundUser User
	result := db.Debug().Where("username = ?", req.Username).First(&foundUser) //db.Where("username = ?", req.Username).First(&foundUser)
	if result.Error == gorm.ErrRecordNotFound {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
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
	db, err := connectDB()

	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	var userID = r.URL.Query().Get("userid")

	var messages []Message

	err = db.Table("messages").
		// Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON users.user_id = messages.author_id").
		Where("flagged = ? AND (author_id = ? OR author_id IN (SELECT who_id FROM followers WHERE whom_id = ?))", false, userID, userID).
		Order("messages.pub_date DESC").
		Limit(PER_PAGE).
		Find(&messages).Error

	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	// Convert to APIMessage format
	var apiMessages []APIMessage
	for _, msg := range messages {
		apiMessages = append(apiMessages, APIMessage{
			Content: msg.Text,
			PubDate: msg.PubDate,
			User:    msg.Author.Username,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiMessages)

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

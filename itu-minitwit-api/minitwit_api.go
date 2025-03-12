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
	"gorm.io/gorm/logger"
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

	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info)})
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
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	UpdateLatest(r)
	vars := mux.Vars(r)

	// Get user by username
	var user User
	if err := db.Where("username = ?", vars["username"]).First(&user).Error; err != nil {
		RespondJSONError(w, http.StatusNotFound, "Cannot find user")
		return
	}

	// Retrieve usernames of people this user follows
	var followerUsernames []string
	err = db.Table("users").
		Joins("JOIN followers ON followers.whom_id = users.user_id").
		Where("followers.who_id = ?", user.UserID).
		Limit(GetNumberHandler(r)).
		Pluck("users.username", &followerUsernames).Error
	if err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Failed to retrieve follows list")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"follows": followerUsernames})
}

func POSTFollowerHandler(w http.ResponseWriter, r *http.Request) {
	/* TODO - use orm instead of query
	UpdateLatest(r)
	// if NotReqFromSimulator(w, r) {
	// 	return
	// }

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	defer db.Close()

	vars := mux.Vars(r)

	userID, _ := getUserID(db, vars["username"])

	if userID == -1 {
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	var data map[string]interface{}

	json.NewDecoder(r.Body).Decode(&data)

	if followsUsername, exists := data["follow"]; exists {
		followsUserID, _ := getUserID(db, followsUsername.(string))
		if followsUserID == -1 {
			http.Error(w, "The user you are trying to follow cannot be found", http.StatusNotFound)
			return
		}
		query := `INSERT INTO follower (who_id, whom_id) VALUES (?, ?)`

		res, _ := db.Exec(query, userID, followsUserID)

		lastInsertedID, err := res.LastInsertId()

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			data = map[string]interface{}{
				"status": http.StatusBadRequest,
				"res":    err.Error(),
			}
		} else {
			w.WriteHeader(http.StatusNoContent)
			data = map[string]interface{}{
				"status": http.StatusNoContent,
				"res":    fmt.Sprint(lastInsertedID),
			}
		}
		json.NewEncoder(w).Encode(data)
		return
	} else if unfollowsUsername, exists := data["unfollow"]; exists {
		unfollowsUserID, _ := getUserID(db, unfollowsUsername.(string))
		if unfollowsUserID == -1 {
			http.Error(w, "The user you are trying to unfollow cannot be found", http.StatusNotFound)
			return
		}
		query := `DELETE FROM follower WHERE who_id=? and WHOM_ID=?`
		res, _ := db.Exec(query, userID, unfollowsUserID)

		lastInsertedID, err := res.LastInsertId()

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			data = map[string]interface{}{
				"status": http.StatusBadRequest,
				"res":    err.Error(),
			}
		} else {
			w.WriteHeader(http.StatusNoContent)
			data = map[string]interface{}{
				"status": http.StatusNoContent,
				"res":    fmt.Sprint(lastInsertedID),
			}
		}
		json.NewEncoder(w).Encode(data)
		return
	}
	*/
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	UpdateLatest(r)

	var data map[string]string

	if r.Body == nil {
		RespondJSONError(w, http.StatusBadRequest, "Empty request body")
		return
	}
	defer r.Body.Close()

	// JSON decoding errors properly
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		RespondJSONError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	username := strings.TrimSpace(data["username"])
	email := strings.TrimSpace(data["email"])
	pwd := data["pwd"]

	if username == "" {
		RespondJSONError(w, http.StatusBadRequest, "You have to enter a username")
		return
	}
	if email == "" || !strings.Contains(email, "@") {
		RespondJSONError(w, http.StatusBadRequest, "You have to enter a valid email address")
		return
	}
	if pwd == "" {
		RespondJSONError(w, http.StatusBadRequest, "You have to enter a password")
		return
	}

	// Check if username is already taken
	var existing User
	err = db.Where("username = ?", username).First(&existing).Error
	if err == nil {
		RespondJSONError(w, http.StatusBadRequest, "The username is already taken")
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		RespondJSONError(w, http.StatusInternalServerError, "Failed to check existing user")
		return
	}

	newUser := User{
		Username: username,
		Email:    email,
		PWHash:   pwd,
	}
	if err := db.Create(&newUser).Error; err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User registered successfully",
	})
}

func GETAllMessagesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	UpdateLatest(r)

	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = false").
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error
	if err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Failed to retrieve messages")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func GETUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	UpdateLatest(r)
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		RespondJSONError(w, http.StatusBadRequest, "Missing username parameter")
		return
	}

	// Fetch the user ID (to ensure user exists)
	userID, err := getUserID(db, username)
	if err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Error retrieving user")
		return
	}
	if userID == 0 {
		RespondJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = false AND users.username = ?", username).
		Order("messages.pub_date DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error
	if err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Failed to retrieve messages")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func POSTMessagesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	UpdateLatest(r)
	vars := mux.Vars(r)

	username := vars["username"]
	if username == "" {
		RespondJSONError(w, http.StatusBadRequest, "Missing username parameter")
		return
	}

	// Get user ID for the author of the message
	userID, err := getUserID(db, username)
	if err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Error retrieving user")
		return
	}
	if userID == 0 {
		RespondJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		RespondJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	content, ok := data["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		RespondJSONError(w, http.StatusBadRequest, "Content is required")
		return
	}

	message := Message{
		AuthorID: userID,
		Text:     content,
		PubDate:  time.Now().Unix(),
		Flagged:  false,
	}
	if err := db.Create(&message).Error; err != nil {
		RespondJSONError(w, http.StatusInternalServerError, "Failed to create message")
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
	/* TODO - use orm instead of query
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	whoUsername := r.URL.Query().Get("whoUsername")
	whomUsername := r.URL.Query().Get("whomUsername")
	whoUsernameID, _ := getUserID(db, whoUsername)
	whomUsernameID, _ := getUserID(db, whomUsername)
	var isFollowing bool
	err = db.QueryRow(
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
	}
	json.NewEncoder(w).Encode(isFollowing)
	*/
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
	/* TODO - use orm instead of query
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var userID = r.URL.Query().Get("userid")
	rows, err := db.Query(`
	SELECT  m.text, m.pub_date, u.username
	FROM message m, user u
	WHERE m.flagged = 0 AND u.user_id = m.author_id
	AND (m.author_id = ? OR m.author_id IN (
		SELECT who_id FROM follower WHERE whom_id = ?
		))
		ORDER BY m.pub_date DESC LIMIT ?`, userID, userID, PER_PAGE)

	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect messages
	var messages []APIMessage
	for rows.Next() {
		var msg APIMessage
		if err := rows.Scan(
			&msg.Content, &msg.PubDate, &msg.User,
		); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating over rows", http.StatusInternalServerError)
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
	*/
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

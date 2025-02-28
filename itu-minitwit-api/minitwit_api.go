package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const DATABASE = "minitwit.db"
const PER_PAGE = 30

//const PER_PAGE = 30 //useful for the html template but not for the API implementation

// var db *sql.DB

// connectDB opens a connection to the SQLite3 database
func connectDB() (*sql.DB, error) {
	databasePath := os.Getenv("DATABASE")
	if databasePath == "" {
		databasePath = DATABASE // Fallback in case the env variable is missing
	}
	return sql.Open("sqlite3", databasePath)
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
}

func POSTFollowerHandler(w http.ResponseWriter, r *http.Request) {
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
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	defer db.Close()

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

		if userId != -1 {
			error = "The username is already taken"
		} else {
			// Insert new user into the database
			_, err := db.Exec("INSERT INTO user (username, email, pw_hash) VALUES (?, ?, ?)",
				username, email, HashPassword(password),
			)
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
	fmt.Print(error)
	response := map[string]interface{}{
		"status":    status,
		"error_msg": error,
	}
	json.NewEncoder(w).Encode(response)
}

func GETAllMessagesHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	//number of requested messages
	rowNums := GetNumberHandler(r)
	//update latest param
	UpdateLatest(r)

	query := `
		SELECT  message.text, message.pub_date, user.username
		FROM message, user
        WHERE message.flagged = 0 AND message.author_id = user.user_id
        ORDER BY message.pub_date DESC LIMIT ?`

	rows, err := db.Query(query, rowNums)
	if err != nil {
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

	//response := map[string][]Message{"messages": messages}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredMsgs)
}

func GETUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	//update latest param
	UpdateLatest(r)
	//number of requested messages
	rowNums := GetNumberHandler(r)

	query := `	SELECT  message.text, message.pub_date, user.username
				FROM message, user
				WHERE message.flagged = 0 AND
				user.user_id = message.author_id AND user.user_id = ?
				ORDER BY message.pub_date DESC LIMIT ?`

	vars := mux.Vars(r)

	userID, _ := getUserID(db, vars["username"])

	if userID == -1 {
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	rows, err := db.Query(query, userID, rowNums)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)

		return
	}
	defer rows.Close()

	// Collect USER messages
	var messages []APIMessage
	for rows.Next() {
		var msg APIMessage
		if err := rows.Scan(
			&msg.Content, &msg.PubDate, &msg.User,
		); err != nil {
			fmt.Println(err.Error())
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
}

func POSTMessagesHandler(w http.ResponseWriter, r *http.Request) {
	UpdateLatest(r)

	if NotReqFromSimulator(w, r) {
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	defer db.Close()

	vars := mux.Vars(r)

	userID, _ := getUserID(db, vars["username"])

	if userID == -1 {
		fmt.Printf("Cannot find user: %s", vars["username"])
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)

	content := data["content"].(string)

	query := `INSERT INTO message (author_id, text, pub_date, flagged) VALUES (?, ?, ?, 0)`
	_, err = db.Exec(query, userID, content, FormatDateTime(time.Now().Unix()))

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
			"res":    "",
		}
	}
	json.NewEncoder(w).Encode(data)
}
func GETUserDetailsHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")
	var userDetailsRow *sql.Row
	query := `SELECT user_id, username, email FROM user WHERE `

	if userID != "" {
		query += "user_id = ?"
		userDetailsRow = db.QueryRow(query, userID)
	} else {
		query += "username = ?"
		userDetailsRow = db.QueryRow(query, username)
	}
	var userdetails UserDetails
	err = userDetailsRow.Scan(&userdetails.UserID, &userdetails.Username, &userdetails.Email)
	if err != nil {
		fmt.Print(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(userdetails)
}

func GETFollowingHandler(w http.ResponseWriter, r *http.Request) {
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
}

func PostLoginHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if user exists
	var foundUser LoginRequest
	query := `	SELECT user.username, user.pw_hash
		  		FROM user
		  		WHERE user.username = ?`
	err = db.QueryRow(query, req.Username).Scan(&foundUser.Username, &foundUser.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// At this point we know that a user exists
	// Check the password hash against the one found in the db
	if CheckPasswordHash(req.Password, foundUser.Password) {

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
}
func main() {
	// Create a new mux router
	initDB()

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

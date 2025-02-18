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

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
)

const DATABASE = "minitwit.db"
const PER_PAGE = 30

var db *sql.DB
var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

// connectDB opens a connection to the SQLite3 database
func connectDB() (*sql.DB, error) {
	return sql.Open("sqlite3", DATABASE)
}

// def update_latest(request: request):
//     parsed_command_id = request.args.get("latest", type=int, default=-1)
//     if parsed_command_id != -1:
//         with open("./latest_processed_sim_action_id.txt", "w") as fp:
//             fp.write(str(parsed_command_id))

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

func GetLatestHandler(w http.ResponseWriter, r *http.Request) {
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
	latestID_int, err := strconv.Atoi(latestID)
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

func FollowPageHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Query all followers
	query := `
	SELECT user.username FROM user
	INNER JOIN follower ON follower.whom_id=user.user_id
    WHERE follower.who_id=?
    LIMIT ?`

	vars := mux.Vars(r)
	userId, err := getUserID(db, vars["username"])

	rows, err := db.Query(query, userId, PER_PAGE)
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

	for i, follower := range followers {
		fmt.Fprintf(w, "Index %d: %s\n", i, follower)
	}
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()
	UpdateLatest(r) // Updater the latest parameter

	// If user already in cookies, redirect
	if session.Values["user_id"] != nil {
		fmt.Println(session.Values["user_id"])
		http.Redirect(w, r, "/", http.StatusFound) // TODO: Change to correct redirect
		return
	}

	var error string

	if r.Method == "POST" {
		var data map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		//fmt.Printf("Received: %v", data)
		username := data["username"].(string)
		email := data["email"].(string)
		password := data["pwd"].(string)
		password2 := data["pwd"].(string)

		// Validate input fields
		if username == "" {
			error = "You have to enter a username"
		} else if email == "" {
			error = "You have to enter a valid email address"
		} else if !strings.Contains(email, "@") {
			error = "You have to enter a valid email address"
		} else if password == "" {
			error = "You have to enter a password"
		} else if password != password2 {
			error = "The two passwords do not match"
		} else {
			// Check if the username is already taken
			userId, err := getUserID(db, username)
			if err != nil {
				log.Println("Error retrieving user ID:", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
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

				// Flash message and redirect
				session.AddFlash("You were successfully registered and can login now")
				session.Save(r, w)
				http.Redirect(w, r, "/latest", http.StatusOK)
				return
			}
		}
	}
	fmt.Printf(error)
}

func GetAllMessagesHandler(w http.ResponseWriter, r *http.Request) {
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
	//rowNums := GetNumberHandler(r)

	// select all messages
	query := `
	SELECT message.*, user.* FROM message, user
        WHERE message.flagged = 0 AND message.author_id = user.user_id
        ORDER BY message.pub_date DESC LIMIT ?`

	vars := mux.Vars(r)
	userId, err := getUserID(db, vars["username"])

	rows, err := db.Query(query, userId, PER_PAGE)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect ALL messages NOT DONE YET
	var followers []string
	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		followers = append(followers, follower)
	}

	for i, follower := range followers {
		fmt.Fprintf(w, "Index %d: %s\n", i, follower)
	}

}

func GetUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
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
	//rowNums := GetNumberHandler(r)

	// select all messages from specific user
	query := `
	SELECT message.*, user.* FROM message, user
	WHERE message.flagged = 0 AND
	user.user_id = message.author_id AND user.user_id = ?
	ORDER BY message.pub_date DESC LIMIT ?`

	vars := mux.Vars(r)
	userId, err := getUserID(db, vars["username"])

	rows, err := db.Query(query, userId, PER_PAGE)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect USER messages NOT DONE YET
	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		messages = append(messages, message)
	}
	// print rowNums times
	for i, message := range messages {
		fmt.Fprintf(w, "Index %d: %s\n", i, message)
	}
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

	// Serve static files (e.g., CSS, images, etc.) from the "static" folder
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Define the routes and their handlers
	r.HandleFunc("/latest", GetLatestHandler).Methods("GET")

	// r.HandleFunc("/", TimelineHandler).Methods("GET") // not sure if we should keep this one
	r.HandleFunc("/msgs", GetAllMessagesHandler).Methods("GET")
	r.HandleFunc("/msgs/{username}", GetUserMessagesHandler).Methods("GET")
	// r.HandleFunc("/msgs/{username}", AddMessageHandler).Methods("POST")

	// r.HandleFunc("/login", LoginHandler).Methods("GET", "POST")
	// r.HandleFunc("/logout", LogoutHandler).Methods("GET")
	r.HandleFunc("/register", RegisterHandler).Methods("GET", "POST")

	// // TODO
	r.HandleFunc("/fllws/{username}", FollowPageHandler).Methods("GET")
	// r.HandleFunc("/fllws/{username}", FollowHander).Methods("POST")

	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

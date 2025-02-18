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
	userId, _ := getUserID(db, vars["username"])

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
	}else {
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
			http.Redirect(w, r, "/latest", http.StatusOK)
			return
		}
	}
	var status int

	if error == "" {
		w.WriteHeader(http.StatusOK)
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

func POSTFollowerHandler(w http.ResponseWriter, r *http.Request)  {
	UpdateLatest(r)

	notFromSim := NotReqFromSimulator(w,r)

	if notFromSim {return}

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	defer db.Close()


	vars := mux.Vars(r)

	userID, _ := getUserID(db,vars["username"])

	if userID == -1{
		http.Error(w,"Cannot find user",http.StatusNotFound)
		return
	}

	var data map[string]interface{}

	json.NewDecoder(r.Body).Decode(&data)

	if followsUsername, exists := data["follow"]; exists{
		followsUserID,_ := getUserID(db,followsUsername.(string))
		if followsUserID == -1{
			http.Error(w,"The user you are trying to follow cannot be found", http.StatusNotFound)
			return
		}
		query := `INSERT INTO follower (who_id, whom_id) VALUES (?, ?)`

		res, err := db.Exec(query,userID,followsUserID)


		lastInsertedID, err := res.LastInsertId()

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			data = map[string]interface{}{
				"status": http.StatusBadRequest,
				"res": err.Error(),
			}
		} else {
			w.WriteHeader(http.StatusNoContent)
			data = map[string]interface{}{
				"status": http.StatusNoContent,
				"res": fmt.Sprint(lastInsertedID),
			}
		}
		json.NewEncoder(w).Encode(data)
		return
	} else if unfollowsUsername, exists := data["follow"]; exists {
		unfollowsUserID,_ := getUserID(db,unfollowsUsername.(string))
		if unfollowsUserID == -1{
			http.Error(w,"The user you are trying to unfollow cannot be found", http.StatusNotFound)
			return
		}
		query := `DELETE FROM follower WHERE who_id=? and WHOM_ID=?`
		res, err := db.Exec(query, userID,unfollowsUserID)

		lastInsertedID, err := res.LastInsertId()

	
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			data = map[string]interface{}{
				"status": http.StatusBadRequest,
				"res": err.Error(),
			}
		} else {
			w.WriteHeader(http.StatusNoContent)
			data = map[string]interface{}{
				"status": http.StatusNoContent,
				"res": fmt.Sprint(lastInsertedID),
			}
		}
		json.NewEncoder(w).Encode(data)
		return
	}

}

func GETFollowerHandler(w http.ResponseWriter, r *http.Request)  {
	UpdateLatest(r)

	notFromSim := NotReqFromSimulator(w,r)

	if notFromSim {return}

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	defer db.Close()


	vars := mux.Vars(r)

	userID, _ := getUserID(db,vars["username"])

	if userID == -1{
		http.Error(w,"Cannot find user",http.StatusNotFound)
		return
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
	//r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Define the routes and their handlers
	r.HandleFunc("/latest", GetLatestHandler).Methods("GET")
	r.HandleFunc("/register", RegisterHandler).Methods("POST")
	r.HandleFunc("/fllws/{username}", POSTFollowerHandler).Methods("POST")
	r.HandleFunc("/fllws/{username}", GETFollowerHandler).Methods("GET")
	// r.HandleFunc("/", TimelineHandler).Methods("GET") // not sure if we should keep this one
	// r.HandleFunc("/msgs", PublicTimelineHandler).Methods("GET")
	// r.HandleFunc("/msgs/{username}", UserTimelineHandler).Methods("GET")
	// r.HandleFunc("/msgs/{username}", AddMessageHandler).Methods("POST")

	// r.HandleFunc("/login", LoginHandler).Methods("GET", "POST")
	// r.HandleFunc("/logout", LogoutHandler).Methods("GET")

	// // TODO
	// r.HandleFunc("/fllws/{username}", FollowPageHandler).Methods("GET")
	// r.HandleFunc("/fllws/{username}", FollowHander).Methods("POST")

	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

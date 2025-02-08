package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
)

const DATABASE = "tmp/minitwit.db"
const PER_PAGE = 10


var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

// Connect to the SQLite database
func connectDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", DATABASE)
	if err != nil {
		return nil, fmt.Errorf("could not connect to the database: %v", err)
	}
	return db, nil
}


// Fetch the TimelineHandler messages
func TimelineHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()
	session, _ := store.Get(r, "session-name")
	session.Values["user"] = "Test"
	session.Save(r,w)
	// Query the database for messages
	rows, err := db.Query(`
		SELECT message_id, author_id, text, pub_date, flagged
		FROM message
		WHERE flagged = 0
		ORDER BY pub_date DESC
		LIMIT ?`, PER_PAGE)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Slice to hold the messages
	var messages []Message

	// Loop through rows and scan into Message struct
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.MessageID, &msg.AuthorID, &msg.Text, &msg.PubDate, &msg.Flagged); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		messages = append(messages, msg)
	}

	// Check for any errors after iterating through rows
	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading rows", http.StatusInternalServerError)
		return
	}

	// Print the messages (for simplicity)
	fmt.Fprintf(w, "Timeline:\n")
	for _, msg := range messages {
		fmt.Fprintf(w, "Message ID: %d, Author ID: %d, Text: %s, Pub Date: %d, Flagged: %d\n",
			msg.MessageID, msg.AuthorID, msg.Text, msg.PubDate, msg.Flagged)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request){
	session, _ := store.Get(r, "session-name")
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// If user is already in the cookies, just redirect
	if session.Values["user"] != nil {
		http.Redirect(w,r,"/",http.StatusFound) // TODO: Change to correct redirect
	}

	var error string
	if r.Method == "POST" {
		var user User
		row := db.QueryRow("select * from user where username = ?",r.FormValue("username"))
		// Check if user exists
		if err := row.Scan(&user.UserID, &user.Username, &user.Email, &user.PWHash) ; err != nil {
			error = "Invalid username"
			//http.Error(w, "Invalid username", http.StatusInternalServerError)
		// Check if password is correct
		} else if !CheckPasswordHash(r.FormValue("password"),user.PWHash) {
			error = "Invalid password"
			//http.Error(w, "Invalid password", http.StatusInternalServerError)
		// Redirect and save user_id in cookies if the above checks failed
		} else {
			session.Values["user_id"] = user.UserID
			session.Save(r,w)
			http.Redirect(w,r,"/",http.StatusFound)
		}
	}
	fmt.Fprintf(w,"This should be a Login page :), with the following error %s", error)
}

func RegisterHandle(w http.ResponseWriter, r *http.Request)  {
	session, _ := store.Get(r, "session-name")
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()
	// If user already in cookies, redirect
	if session.Values["user"] != nil {
		http.Redirect(w,r,"/",http.StatusFound) // TODO: Change to correct redirect
	}
	var error string
	if r.Method == "POST" {
		// If there is username in the form
		if r.FormValue("username") == "" {
			error = "You have to enter a username"
		// If the email is missing
		} else if r.FormValue("email") == "" {
			error = "You have to enter a valid email address"
		// If the email is address is invalid
		} else if !strings.Contains(r.FormValue("email"),"@"){
			error = "You have to enter a valid email address"
		// If there is not a password
		} else if r.FormValue("password") == "" {
			error = "You have to enter a password"
		// If the two passwords do not match
		} else if r.FormValue("password") != r.FormValue("password2") {
			error = "The two passwords do not match"
			} else  {
				userId, err := getUserID(db,r.FormValue("username"))
				if err != nil {
					log.Println("Error retrieving user ID:", err)
				}
			// If the username is already taken
			if userId != -1 {
				error = "The username is already taken"
			// If the form is correct
			} else {
				res, err := db.Exec("insert into user (	username, email, pw_hash) values (?, ?, ?)", 
					r.FormValue("username"),
					r.FormValue("email"),
					HashPassword(r.FormValue("password")),
				)
				if err != nil {
					fmt.Println("This is bad")
					log.Println(err)
				}
				fmt.Println(res.LastInsertId())
				http.Redirect(w,r,"/login",http.StatusAccepted)
			}
		}
	}
	fmt.Fprintf(w, "This should be a register page with the following error: %s", error)

}

func main() {
	// Create a new mux router
	r := mux.NewRouter()

	// Define the routes and their handlers
	r.HandleFunc("/", TimelineHandler).Methods("GET")
	r.HandleFunc("/login", LoginHandler).Methods("GET","POST")
	r.HandleFunc("/register", RegisterHandle).Methods("GET","POST")
	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

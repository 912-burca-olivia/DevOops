package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
)

const DATABASE = "tmp/minitwit.db"
const PER_PAGE = 10

// Template cache
//var templates = template.Must(template.ParseGlob("templates/*.html"))

// var templates = template.Must(template.ParseFiles(

// 	"templates/layout.html",
// 	"templates/timeline.html",
// 	"templates/login.html",
// 	"templates/register.html",

// ))
var db *sql.DB
var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

// Gravatar function that generates the Gravatar URL based on the email
func gravatar(email string, size int) string {
	// Clean up the email and hash it with MD5
	email = strings.TrimSpace(email)
	hash := md5.New()
	hash.Write([]byte(strings.ToLower(email)))
	return fmt.Sprintf("https://www.gravatar.com/avatar/%s?d=identicon&s=%d", hex.EncodeToString(hash.Sum(nil)), size)
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}, useGravatar bool) {
	// Set up a FuncMap and conditionally add gravatar function if needed
	// funcMap := template.FuncMap{}
	// if useGravatar {
	// 	funcMap["gravatar"] = gravatar
	// }

	// err := templates.ExecuteTemplate(w, tmpl, data)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }

	var t = template.Must(template.ParseFiles("templates/"+tmpl+".html", "templates/layout.html"))

	err := t.ExecuteTemplate(w, "layout", data)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", DATABASE) // Change this if using MySQL/PostgreSQL
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	// Verify connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	log.Println("Database connected successfully")
	return nil
}

// Close the database connection on shutdown
func closeDB() {
	if db != nil {
		db.Close()
		log.Println("Database connection closed")
	}
}

// Fetch the TimelineHandler messages
func TimelineHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("We got a visitor from:", r.RemoteAddr)

	session, _ := store.Get(r, "session-name")
	// Check if user is logged in
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/public_timeline", http.StatusFound)
		return
	}

	// Query the database for messages
	rows, err := db.Query(`
		SELECT m.message_id, m.author_id, m.text, m.pub_date, m.flagged, u.username, u.email
		FROM message m, user u
		WHERE m.flagged = 0 AND u.user_id = m.author_id
		AND (m.author_id = ? OR m.author_id IN (
	        SELECT whom_id FROM follower WHERE who_id = ?
	    ))
	    ORDER BY m.pub_date DESC LIMIT ?`, userID, userID, PER_PAGE)
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
		if err := rows.Scan(&msg.MessageID, &msg.AuthorID, &msg.Text, &msg.PubDate, &msg.Flagged, &msg.Username, &msg.Email); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		messages = append(messages, msg)
	}

	// Render template
	renderTemplate(w, "timeline", map[string]interface{}{"messages": messages}, false)

}

func PublicTimelineHandler(w http.ResponseWriter, r *http.Request) {
	// Query all public messages
	query := `
        SELECT m.message_id, m.author_id, m.text, m.pub_date, m.flagged, u.username, u.email
        FROM message m, user u
        WHERE m.flagged = 0 AND m.author_id = u.user_id
        ORDER BY m.pub_date DESC LIMIT ?`

	rows, err := db.Query(query, PER_PAGE)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect messages
	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.MessageID, &msg.AuthorID, &msg.Text, &msg.PubDate, &msg.Flagged, &msg.Username, &msg.Email); err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		messages = append(messages, msg)
	}

	// Render template
	renderTemplate(w, "timeline", map[string]interface{}{"messages": messages}, true)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// If user is already in the cookies, just redirect
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/", http.StatusFound) // TODO: Change to correct redirect
	}

	var error string
	if r.Method == "POST" {
		var user User
		row := db.QueryRow("select * from user where username = ?", r.FormValue("username"))
		// Check if user exists
		if err := row.Scan(&user.UserID, &user.Username, &user.Email, &user.PWHash); err != nil {
			error = "Invalid username"
			renderTemplate(w, "login", map[string]interface{}{
				"Error": error,
			}, false)
			return
			// Check if password is correct
		} else if !CheckPasswordHash(r.FormValue("password"), user.PWHash) {
			error = "Invalid password"
			renderTemplate(w, "login", map[string]interface{}{
				"Error": error,
			}, false)
			return

			// Redirect and save user_id in cookies if the above checks failed
		} else {
			session.Values["user_id"] = user.UserID
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}
	renderTemplate(w, "login", nil, false)
}

func RegisterHandle(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// If user already in cookies, redirect
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/", http.StatusFound) // TODO: Change to correct redirect
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
		} else if !strings.Contains(r.FormValue("email"), "@") {
			error = "You have to enter a valid email address"
			// If there is not a password
		} else if r.FormValue("password") == "" {
			error = "You have to enter a password"
			// If the two passwords do not match
		} else if r.FormValue("password") != r.FormValue("password2") {
			error = "The two passwords do not match"
		} else {
			userId, err := getUserID(db, r.FormValue("username"))
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
				http.Redirect(w, r, "/login", http.StatusAccepted)
			}
		}
	}

	data := map[string]interface{}{
		"Error":    error,
		"Username": r.FormValue("username"),
		"Email":    r.FormValue("email"),
	}
	renderTemplate(w, "register", data, false)
}

func AddMessageHandler(w http.ResponseWriter, r *http.Request) {
	// Get the current session
	session, _ := store.Get(r, "session-name")

	// Check if the user is logged in
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if the message text is provided
	messageText := r.FormValue("text")
	if messageText == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	// Get current timestamp
	pubDate := int(time.Now().Unix())

	// Insert the message into the database
	_, err := db.Exec(`INSERT INTO message (author_id, text, pub_date, flagged)
	                     VALUES (?, ?, ?, 0)`, userID, messageText, pubDate)
	if err != nil {
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	// Flash message (you can implement this with a session or redirect with a query parameter)
	http.Redirect(w, r, "/timeline", http.StatusFound)
}

func main() {
	// Create a new mux router
	r := mux.NewRouter()

	// Initialize DB
	if err := initDB(); err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer closeDB() // Ensure DB is closed when the app exits

	// Serve static files (e.g., CSS, images, etc.) from the "static" folder
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Define the routes and their handlers
	r.HandleFunc("/", TimelineHandler).Methods("GET")
	r.HandleFunc("/public_timeline", PublicTimelineHandler).Methods("GET")
	r.HandleFunc("/login", LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/register", RegisterHandle).Methods("GET", "POST")
	r.HandleFunc("/add_message", AddMessageHandler).Methods("POST")
	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

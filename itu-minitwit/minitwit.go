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

const DATABASE = "minitwit.db"
const PER_PAGE = 30

var db *sql.DB
var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

// Gravatar function that generates the Gravatar URL based on the email
func Gravatar(size int, email string) string {
	// Clean up the email and hash it with MD5
	email = strings.TrimSpace(email)
	hash := md5.New()
	hash.Write([]byte(strings.ToLower(email)))
	return fmt.Sprintf("https://www.gravatar.com/avatar/%s?d=identicon&s=%d", hex.EncodeToString(hash.Sum(nil)), size)
}


func FormatDateTime(timestamp int64) string {
	t := time.Unix(timestamp, 0)
	return t.Format("Jan 2, 2006 at 3:04PM")
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {

	tmpls := template.New("").Funcs(template.FuncMap{
		"FormatDateTime": FormatDateTime,
		"Gravatar" : Gravatar,
	})

	tmpls, err := tmpls.ParseFiles("templates/"+tmpl+".html", "templates/layout.html")

	if err != nil {
		http.Error(w, "Error parsing templates", http.StatusInternalServerError)
		return
	}

	err = tmpls.ExecuteTemplate(w, "layout", data)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// connectDB opens a connection to the SQLite3 database
func connectDB() (*sql.DB, error) {
	return sql.Open("sqlite3", DATABASE)
}

// Fetch the TimelineHandler messages
func TimelineHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("We got a visitor from:", r.RemoteAddr)

	session, _ := store.Get(r, "session-name")

	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Check if user is logged in
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/public_timeline", http.StatusFound)
		return
	}

	// Query the database for user details
	var user User
	err = db.QueryRow(`SELECT user_id, username, email FROM user WHERE user_id = ?`, userID).
		Scan(&user.UserID, &user.Username, &user.Email)
	if err != nil {
		http.Error(w, "Failed to fetch user details", http.StatusInternalServerError)
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

	flashes := session.Flashes() // Get flash messages
	session.Save(r, w)

	// Render template
	renderTemplate(w, "timeline", map[string]interface{}{
		"User":     user,
		"messages": messages,
		"Flashes":  flashes,
		"Endpoint": "timeline",
	})

}

func PublicTimelineHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	userID, ok := session.Values["user_id"].(int)
	var user User
	if ok {
		// Query the database for user details
		err = db.QueryRow(`SELECT user_id, username, email FROM user WHERE user_id = ?`, userID).
			Scan(&user.UserID, &user.Username, &user.Email)
		if err != nil {
			http.Error(w, "Failed to fetch user details", http.StatusInternalServerError)
			return
		}
	}

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

	flashes := session.Flashes() // Get flash messages
	session.Save(r, w)           // Clear them after retrieval

	// Render template depending on whether the user is logged in or not

	if !ok {
		renderTemplate(w, "timeline", map[string]interface{}{
			"messages": messages,
			"Flashes":  flashes,
			"Endpoint": "public_timeline",
		})
	} else {
		renderTemplate(w, "timeline", map[string]interface{}{
			"messages": messages,
			"Flashes":  flashes,
			"User":     user,
			"Endpoint": "public_timeline",
		})
	}

}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// If user is already in the cookies, just redirect
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
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
			})
			return
			// Check if password is correct
		} else if !CheckPasswordHash(r.FormValue("password"), user.PWHash) {
			error = "Invalid password"
			renderTemplate(w, "login", map[string]interface{}{
				"Error": error,
			})
			return

			// Redirect and save user_id in cookies if the above checks failed
		} else {
			session.AddFlash("You were logged in")
			session.Values["user_id"] = user.UserID
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	flashes := session.Flashes()
	session.Save(r, w)

	renderTemplate(w, "login", map[string]interface{}{
		"Flashes": flashes,
	})
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

	// If user already in cookies, redirect
	if session.Values["user_id"] != nil {
		fmt.Println(session.Values["user_id"])
		fmt.Println("We went into the dark place")
		http.Redirect(w, r, "/", http.StatusFound) // TODO: Change to correct redirect
		return
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
				return
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
				session.AddFlash("You were successfully registered and can login now")
				session.Save(r, w)
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
		}
	}

	data := map[string]interface{}{
		"Error":    error,
		"Username": r.FormValue("username"),
		"Email":    r.FormValue("email"),
	}
	renderTemplate(w, "register", data)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	delete(session.Values, "user_id")
	session.AddFlash("You were logged out")
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func AddMessageHandler(w http.ResponseWriter, r *http.Request) {
	// Get the current session
	session, _ := store.Get(r, "session-name")

	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

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
	_, err = db.Exec(`INSERT INTO message (author_id, text, pub_date, flagged)
	                     VALUES (?, ?, ?, 0)`, userID, messageText, pubDate)
	if err != nil {
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	session.AddFlash("Your message was recorded")
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func FollowHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	db, err := connectDB()
	vars := mux.Vars(r)
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()
	if session.Values["user_id"] == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}
	whom_id, err := getUserID(db, vars["username"])
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	if whom_id == -1 {
		http.Error(w, "User does not exist", http.StatusNotFound)
		return
	}
	db.Exec("insert into follower (who_id, whom_id) values (?, ?)",
		session.Values["user_id"],
		whom_id)
	session.AddFlash("You are now following " + vars["username"]) // TODO: Don't know if working
	session.Save(r, w)
	http.Redirect(w, r, fmt.Sprintf("/user_timeline/%s", vars["username"]), http.StatusFound)

}

func UnfollowHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	db, err := connectDB()
	vars := mux.Vars(r)
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()
	if session.Values["user_id"] == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}
	whom_id, err := getUserID(db, vars["username"])
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	if whom_id == -1 {
		http.Error(w, "User does not exist", http.StatusNotFound)
		return
	}
	db.Exec("delete from follower where who_id=? and whom_id=?",
		session.Values["user_id"],
		whom_id)
	session.AddFlash("You are no longer following " + vars["username"]) // TODO: Don't know if working
	session.Save(r, w)
	http.Redirect(w, r, fmt.Sprintf("/user_timeline/%s", vars["username"]), http.StatusFound)
}

func UserTimelineHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	session, _ := store.Get(r, "session-name")
	vars := mux.Vars(r)
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	userID, ok := session.Values["user_id"].(int)
	var user User
	if ok {
		// Query the database for user details
		err = db.QueryRow(`SELECT user_id, username, email FROM user WHERE user_id = ?`, userID).
			Scan(&user.UserID, &user.Username, &user.Email)
		if err != nil {
			http.Error(w, "Failed to fetch user details", http.StatusInternalServerError)
			return
		}
	}

	var profile_user User
	err = db.QueryRow("select * from user where username = ?", vars["username"]).Scan(&profile_user.UserID, &profile_user.Username, &profile_user.Email, &profile_user.PWHash)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
		}
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
	}
	var followed bool
	if session.Values["user_id"] != nil {
		//var isFollowed int
		err = db.QueryRow(
			`select 1 
			from follower 
			where follower.who_id = ? and follower.whom_id = ?`,
			session.Values["user_id"],
			profile_user.UserID).
			Scan(&followed)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Println("User is not following") // Return -1 if no user is found
			} else {
				http.Error(w, "Database connection failed", http.StatusInternalServerError)
			}
		}
	}

	fmt.Println("We have reached")
	// Query the database for messages
	// Removed user.* from Select
	rows, err := db.Query(`
	SELECT message.message_id, message.author_id, message.text, message.pub_date, message.flagged, user.username, user.email
	FROM message, user
	WHERE user.user_id = message.author_id and user.user_id = ?
	ORDER BY message.pub_date DESC
	LIMIT ?`, profile_user.UserID, PER_PAGE)
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

	flashes := session.Flashes() // Get flash messages
	session.Save(r, w)           // Clear them after retrieval

	// render template based on whether user is logged in or not
	if ok {
		renderTemplate(w, "timeline", map[string]interface{}{
			"User":        user,
			"ProfileUser": profile_user,
			"Followed":    followed,
			"messages":    messages,
			"Endpoint":    "user_timeline",
			"Flashes":     flashes,
		})
	} else {
		renderTemplate(w, "timeline", map[string]interface{}{
			"ProfileUser": profile_user,
			"Followed":    followed,
			"messages":    messages,
			"Endpoint":    "user_timeline",
			"Flashes":     flashes,
		})
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
	r.HandleFunc("/", TimelineHandler).Methods("GET")
	r.HandleFunc("/public_timeline", PublicTimelineHandler).Methods("GET")
	r.HandleFunc("/login", LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/register", RegisterHandler).Methods("GET", "POST")
	r.HandleFunc("/add_message", AddMessageHandler).Methods("POST")
	r.HandleFunc("/logout", LogoutHandler).Methods("GET")
	r.HandleFunc("/{username}/follow", FollowHandler).Methods("GET")
	r.HandleFunc("/{username}/unfollow", UnfollowHandler).Methods("GET")
	r.HandleFunc("/user_timeline/{username}", UserTimelineHandler).Methods("GET")

	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// Database path
const DATABASE = "/tmp/minitwit.db"
const PER_PAGE = 30

type Message struct {
	MessageID int    `json:"message_id"`
	AuthorID  int    `json:"author_id"`
	Text      string `json:"text"`
	PubDate   string `json:"pub_date"`
	Flagged   int    `json:"username"`
}

type User struct {
	ID    int
	Name  string
	Email string
}

// App struct holds the router and database connection
type App struct {
	Router *mux.Router
	DB     *sql.DB
}

// Connect to the database
func connectDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", DATABASE)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Initialize the database by running the schema
func initDB() {
	db, err := connectDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Read the schema.sql file
	// sqlBytes, err := os.ReadFile("schema.sql")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // Execute the SQL statements from the file
	// _, err = db.Exec(string(sqlBytes))
	// if err != nil {
	// 	log.Fatal(err)
	// }

	fmt.Println("Database initialized")
}

// Query the database and return a list of users
func queryDB(query string, args ...interface{}) ([]User, error) {
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func timeline(w http.ResponseWriter, r *http.Request) {
	// Print request details like Flask's request.remote_addr
	fmt.Println("We got a visitor from: " + r.RemoteAddr)

	// Check if user is logged in
	// userID := r.URL.Query().Get("user_id") // Example: getting the user_id from query params
	// if userID == "" {
	// 	// Redirect to public timeline if no user is logged in
	// 	http.Redirect(w, r, "/public_timeline", http.StatusFound)
	// 	return
	// }

	// Example of offset from URL query parameters
	//offset := r.URL.Query().Get("offset")

	// Query the database (same query as in your Python example)
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Prepare the query
	rows, err := db.Query(`
		select message.*, user.* from message, user
        where message.flagged = 0 and message.author_id = user.user_id
        order by message.pub_date desc limit ?`, PER_PAGE)
	if err != nil {
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Placeholder for the messages you get from the query
	var messages []string
	for rows.Next() {
		var message, user string
		err := rows.Scan(&message, &user)
		if err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		messages = append(messages, message) // For simplicity, using just the message
	}

	// Render the template (you can use Go's html/template or another templating engine)
	// Here, we'll just print out the messages
	fmt.Fprintf(w, "Timeline:\n")
	for _, msg := range messages {
		fmt.Fprintf(w, "%s\n", msg)
	}
}

func main() {
	// Create a new mux router
	r := mux.NewRouter()

	// Define the routes and their handlers
	r.HandleFunc("/", timeline).Methods("GET")

	initDB()

	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

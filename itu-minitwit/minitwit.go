package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const DATABASE = "tmp/minitwit.db"
const PER_PAGE = 10

// Define the Message struct
type Message struct {
	MessageID int    `json:"message_id"`
	AuthorID  int    `json:"author_id"`
	Text      string `json:"text"`
	PubDate   int    `json:"pub_date"`
	Flagged   int    `json:"flagged"`
}

// Connect to the SQLite database
func connectDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", DATABASE)
	if err != nil {
		return nil, fmt.Errorf("could not connect to the database: %v", err)
	}
	return db, nil
}

// Fetch the timeline messages
func timeline(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

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

func main() {
	// Create a new mux router
	r := mux.NewRouter()

	// Define the routes and their handlers
	r.HandleFunc("/", timeline).Methods("GET")

	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

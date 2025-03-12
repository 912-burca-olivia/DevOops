package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
)

var ENDPOINT = "http://localhost:9090"

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
		"Gravatar":       Gravatar,
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

	// Get user data
	var userDetails UserDetails
	err := getUserDetailsByID(w, userID, &userDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	baseURL := fmt.Sprintf("%s/followingmsgs", ENDPOINT)
	u, err := url.Parse(baseURL)
	if err != nil {
		fmt.Print(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	queryParams := url.Values{}
	queryParams.Add("userid", strconv.Itoa(userDetails.UserID))
	u.RawQuery = queryParams.Encode()
	u.Query()

	// ✅ Log the API URL being requested
	log.Println("🟢 Fetching messages from:", u.String())

	// Send request to API
	res, err := http.Get(u.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	// ✅ Log the HTTP status
	log.Println("🟢 API Response Status:", res.Status)

	// Read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	// ✅ Log the raw response body
	log.Println("🟢 Raw API Response:", string(body))

	// Parse response into `messages`
	var messages []Message
	err = json.Unmarshal(body, &messages)
	if err != nil {
		log.Println("🔴 Error unmarshalling JSON:", err)
	}

	flashes := session.Flashes()
	session.Save(r, w)

	// Render template
	renderTemplate(w, "timeline", map[string]interface{}{
		"User":     userDetails,
		"username": userID,
		"messages": messages,
		"Flashes":  flashes,
		"Endpoint": "timeline",
	})
}

func PublicTimelineHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	userID, ok := session.Values["user_id"].(int)

	var userDetails UserDetails

	if ok {
		err := getUserDetailsByID(w, userID, &userDetails)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Query the API for messages
	// Get url
	url := fmt.Sprintf("%s/msgs", ENDPOINT)
	// Send request
	res, err := http.Get(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	var messages []Message
	err = json.Unmarshal(body, &messages)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
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
			"User":     userDetails,
			"Endpoint": "public_timeline",
		})
	}

}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// If user is already in the cookies, just redirect
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == "POST" {
		data := map[string]string{
			"Username": r.FormValue("username"),
			"Password": r.FormValue("password")}

		jsonData, err := json.Marshal(data)

		url := fmt.Sprintf("%s/login", ENDPOINT)

		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
			return
		}

		// Send POST request
		resp, _ := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if resp.StatusCode == http.StatusOK {
			var userdetails UserDetails
			err := getUserDetailsByUsername(w, r.FormValue("username"), &userdetails)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			session.AddFlash("You were logged in")
			session.Values["user_id"] = userdetails.UserID
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		} else {
			if resp.StatusCode == http.StatusUnauthorized {
				renderTemplate(w, "login", map[string]interface{}{
					"Error": "Invalid password",
				})
				return
			} else if resp.StatusCode == http.StatusNotFound {
				renderTemplate(w, "login", map[string]interface{}{
					"Error": "Invalid username",
				})
				return
			} else {
				renderTemplate(w, "login", map[string]interface{}{
					"Error": "Invalid credentials",
				})
				return
			}
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

	// If user already in cookies, redirect
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/public_timline", http.StatusFound) // TODO: Change to correct redirect
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
			var userdetails UserDetails
			err := getUserDetailsByUsername(w, r.FormValue("username"), &userdetails)
			if err == nil {
				error = "The username is already taken"
				log.Println("Error retrieving user ID:", err)
				data := map[string]interface{}{
					"Error":    error,
					"Username": r.FormValue("username"),
					"Email":    r.FormValue("email"),
				}
				renderTemplate(w, "register", data)
				return
			} else {
				url := fmt.Sprintf("%s/register", ENDPOINT) // Adjust based on your server configuration

				// Define request payload
				data := map[string]string{
					"username": r.FormValue("username"),
					"email":    r.FormValue("email"),
					"pwd":      r.FormValue("password"),
				}
				jsonData, _ := json.Marshal(data)
				req, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if req.StatusCode == http.StatusBadRequest {
					error = "Error handling your request"
					log.Println("Error retrieving user ID:", err)
					data := map[string]interface{}{
						"Error":    error,
						"Username": r.FormValue("username"),
						"Email":    r.FormValue("email"),
					}
					renderTemplate(w, "register", data)
					return
				}
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
	session, _ := store.Get(r, "session-name")

	// Check if user is logged in
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch user details
	var userDetails UserDetails
	if err := getUserDetailsByID(w, userID, &userDetails); err != nil {
		http.Error(w, "Failed to retrieve user details", http.StatusBadRequest)
		return
	}

	// Validate message text
	messageText := strings.TrimSpace(r.FormValue("text"))
	if messageText == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	// Prepare API request
	url := fmt.Sprintf("%s/msgs/%s", ENDPOINT, userDetails.Username)
	requestBody, err := json.Marshal(map[string]string{"content": messageText})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Send POST request to API
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		http.Error(w, "Failed to send request to API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read API response
	var apiResponse map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		http.Error(w, "Invalid API response", http.StatusInternalServerError)
		return
	}

	// Ensure successful message posting
	if resp.StatusCode != http.StatusCreated {
		http.Error(w, apiResponse["error_msg"], resp.StatusCode)
		return
	}

	// Flash message and redirect
	session.AddFlash(apiResponse["message"])
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func FollowHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	vars := mux.Vars(r)
	if session.Values["user_id"] == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}
	var userDetails UserDetails
	err := getUserDetailsByID(w, session.Values["user_id"].(int), &userDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	url := fmt.Sprintf("%s/fllws/%s", ENDPOINT, vars["username"])
	data := map[string]string{"follow": userDetails.Username}
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	if resp.StatusCode != 204 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	session.AddFlash("You are now following " + vars["username"]) // TODO: Don't know if working
	session.Save(r, w)
	http.Redirect(w, r, fmt.Sprintf("/user_timeline/%s", vars["username"]), http.StatusFound)

}

func UnfollowHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	vars := mux.Vars(r)
	if session.Values["user_id"] == nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}
	var userDetails UserDetails
	err := getUserDetailsByID(w, session.Values["user_id"].(int), &userDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	url := fmt.Sprintf("%s/fllws/%s", ENDPOINT, vars["username"])
	data := map[string]string{"unfollow": userDetails.Username}
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	if resp.StatusCode != 204 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	session.AddFlash("You are no longer following " + vars["username"]) // TODO: Don't know if working
	session.Save(r, w)
	http.Redirect(w, r, fmt.Sprintf("/user_timeline/%s", vars["username"]), http.StatusFound)

}

func UserTimelineHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	vars := mux.Vars(r)

	userID, ok := session.Values["user_id"].(int)
	var userDetails UserDetails

	if ok {
		if err := getUserDetailsByID(w, userID, &userDetails); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Get profile user details
	var profileUser UserDetails
	if err := getUserDetailsByUsername(w, vars["username"], &profileUser); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch messages from API
	url := fmt.Sprintf("%s/msgs/%s", ENDPOINT, profileUser.Username)
	res, err := http.Get(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, "Failed to read API response", http.StatusInternalServerError)
		return
	}

	var messages []Message
	if err := json.Unmarshal(body, &messages); err != nil {
		http.Error(w, "Failed to parse API response", http.StatusInternalServerError)
		return
	}

	flashes := session.Flashes()
	session.Save(r, w)

	renderTemplate(w, "timeline", map[string]interface{}{
		"User":        userDetails,
		"ProfileUser": profileUser,
		"messages":    messages,
		"Endpoint":    "user_timeline",
		"Flashes":     flashes,
	})
}

func getEndpoint() string {
	defaultEndpoint := "http://localhost:9090" // Default if ENDPOINT is not set
	if endpoint, exists := os.LookupEnv("ENDPOINT"); exists {
		return endpoint
	}
	return defaultEndpoint
}

func main() {
	ENDPOINT = getEndpoint()
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
	r.HandleFunc("/", TimelineHandler).Methods("GET")                             // Done
	r.HandleFunc("/public_timeline", PublicTimelineHandler).Methods("GET")        // Done
	r.HandleFunc("/user_timeline/{username}", UserTimelineHandler).Methods("GET") // Done
	r.HandleFunc("/add_message", AddMessageHandler).Methods("POST")               // Done
	r.HandleFunc("/register", RegisterHandler).Methods("GET", "POST")             // Done
	r.HandleFunc("/login", LoginHandler).Methods("GET", "POST")                   // Done
	r.HandleFunc("/logout", LogoutHandler).Methods("GET")                         // Done
	r.HandleFunc("/{username}/follow", FollowHandler).Methods("GET")
	r.HandleFunc("/{username}/unfollow", UnfollowHandler).Methods("GET")

	// Start the server on port 8080
	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

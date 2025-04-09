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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// const DATABASE = "../minitwit.db"
const PER_PAGE = 30
const USER_NOT_FOUND = "User not found"

var db *gorm.DB
var err error
var port = ":9090"
var store = sessions.NewCookieStore([]byte("SESSION_KEY"))

var logger = logrus.New()

func initLogger() {
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.WarnLevel)
}

type API struct {
	metrics *Metrics
}

func afterRequestLogging(start time.Time, r *http.Request) {
	// Check if a request takes longer than 2 seconds

	duration := time.Since(start)

	if duration > 2*time.Second {
		logger.WithFields(logrus.Fields{
			"method":    r.Method,
			"path":      r.URL.Path,
			"duration":  duration,
			"remote_ip": r.RemoteAddr,
		}).Warn("Slow request detected")
	} else {
		logger.WithFields(logrus.Fields{
			"method":    r.Method,
			"path":      r.URL.Path,
			"duration":  duration,
			"remote_ip": r.RemoteAddr,
		}).Info("Request completed quickly")
	}
}

func connectDB() (*gorm.DB, error) {

	host := os.Getenv("DB_HOST")

	if host == "" {
		dbPath := os.Getenv("DATABASE")
		if dbPath == "" {
			dbPath = "../minitwit.db"
		}
		logger.Info("Connecting to SQLite database", logrus.Fields{"path": dbPath})
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	} else { // postgresql remote
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
		)
		logger.Info("Connecting to PostgreSQL database", logrus.Fields{"host": os.Getenv("DB_HOST")})
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		logger.WithError(err).Error("Failed to connect to the database")
		return nil, err
	}

	logger.Info("Database connection successful")
	return db, nil
}

func FormatDateTime(timestamp int64) string {
	t := time.Unix(timestamp, 0)
	return t.Format("Jan 2, 2006 at 3:04PM")
}

func createDummyUser(api API, username string) (uint, error) {
	// dummy workaround to get rid of errors caused by old api downtime

	// Insert new user into the database
	newUser := User{Username: username, Email: username + "@gmail.com", PWHash: "dummy"}
	err := db.Create(&newUser).Error
	if err != nil {
		log.Println("Error inserting user:", err)
		return 0, err
	}
	log.Println("Added dummy user: ", newUser)
	return api.getUserID(db, username)
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
			defer func() {
				err = file.Close()
				if err != nil {
					fmt.Print(err.Error())
					return
				}
			}()
			_, err = file.WriteString(strconv.Itoa(parsedCommandID))
			if err != nil {
				fmt.Print(err.Error())
				return
			}
		}
	}
}

func (api *API) GETLatestHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)

	logger.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"ip":     r.RemoteAddr,
	}).Info("Get latest request")
	// Read the latest processed action ID from a file
	UpdateLatest(r)
	content, err := os.ReadFile("latest_processed_sim_action_id.txt")
	if err != nil {
		logger.WithError(err).Error("Failed to read latest action ID")
		api.metrics.BadRequests.WithLabelValues("latest").Inc()
		http.Error(w, "Failed to read latest action ID", http.StatusInternalServerError)
		return
	}

	latestID := strings.TrimSpace(string(content))
	if latestID == "" {
		latestID = "-1"
	}

	w.Header().Set("Content-Type", "application/json")
	latestID_int, _ := strconv.Atoi(latestID)
	logger.WithFields(logrus.Fields{
		"latest_id": latestID_int,
	}).Info("Successfully retrieved latest action ID")
	
	err = json.NewEncoder(w).Encode(map[string]int{"latest": latestID_int})
	if err != nil {
		fmt.Print(err.Error())
	}
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

func (api *API) GETFollowerHandler(w http.ResponseWriter, r *http.Request) {
	
	//number of requested followers
	rowNums := GetNumberHandler(r)
	
	start := time.Now()
	defer afterRequestLogging(start, r)

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"query":     r.URL.RawQuery,
		"remote_ip": r.RemoteAddr,
	}).Info("GETFollowerHandler called")

	UpdateLatest(r)
	vars := mux.Vars(r)

	userID, _ := api.getUserID(db, vars["username"])

	if userID == 0 {
		logger.WithField("username", vars["username"]).Warn(USER_NOT_FOUND)
		api.metrics.BadRequests.WithLabelValues("get_follower").Inc()
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	// Query all followers

	var followers []string

	err = db.
		Table("users").
		Select("users.username").
		Joins("INNER JOIN followers ON followers.whom_id = users.user_id").
		Where("followers.who_id = ?", userID).
		Limit(rowNums).
		Pluck("username", &followers).
		Error

	if err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error(), "userID": userID}).Error("Failed to fetch followers")
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	
	logger.WithField("follower_count", len(followers)).Info("Followers retrieved successfully")
	response := map[string][]string{"follows": followers}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Print(err.Error())
	}

=======
	json.NewEncoder(w).Encode(response)
	
	
>>>>>>> origin/main
}

func (api *API) POSTFollowerHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)
	
	UpdateLatest(r)

	vars := mux.Vars(r)

	userID, _ := api.getUserID(db, vars["username"])

	if userID == 0 {
		logger.Warn(USER_NOT_FOUND, logrus.Fields{"username": vars["username"]})
		api.metrics.BadRequests.WithLabelValues("post_follower").Inc()
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	var data map[string]interface{}

	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	if followsUsername, exists := data["follow"]; exists {
		followsUserID, _ := api.getUserID(db, followsUsername.(string))
		if followsUserID == 0 {
			logger.Warn("Follow target user not found", logrus.Fields{"target_user": followsUsername})
			api.metrics.BadRequests.WithLabelValues("post_follower").Inc()
			http.Error(w, "The user you are trying to follow cannot be found", http.StatusNotFound)
			return
		}

		// Insert follow relationship
		follower := Follower{WhoID: userID, WhomID: followsUserID}

		err := db.Create(&follower).Error

		if err != nil {
			logger.WithError(err).Error("Failed to insert follow relationship")
			api.metrics.BadRequests.WithLabelValues("post_follower").Inc()
			http.Error(w, "Failed to follow user", http.StatusBadRequest)
			return
		}
		api.metrics.FollowRequests.WithLabelValues("follow").Inc()
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			fmt.Print(err.Error())
			return
		}
		return
	} else if unfollowsUsername, exists := data["unfollow"]; exists {
		unfollowsUserID, _ := api.getUserID(db, unfollowsUsername.(string))
		if unfollowsUserID == 0 {
			api.metrics.BadRequests.WithLabelValues("post_follower").Inc()
			http.Error(w, "The user you are trying to unfollow cannot be found", http.StatusNotFound)
			return
		}
		// Delete follow relationship
		err := db.Where("who_id = ? AND whom_id = ?", userID, unfollowsUserID).Delete(&Follower{}).Error

		if err != nil {
			api.metrics.BadRequests.WithLabelValues("post_follower").Inc()
			http.Error(w, "Failed to unfollow user", http.StatusBadRequest)
			return
		}

		logger.WithFields(logrus.Fields{
			"user":   vars["username"],
			"target": followsUsername,
		}).Info("User followed successfully")

		api.metrics.UnfollowRequests.WithLabelValues("unfollow").Inc()
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			fmt.Print(err.Error())
			return
		}
		return
	}

}

func (api *API) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)

	UpdateLatest(r) // Updater the latest parameter

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"remote_ip": r.RemoteAddr,
	}).Info("RegisterHandler called")

	var error = ""

	var data map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	username, email, password := data["username"].(string), data["email"].(string), data["pwd"].(string)

	logger.WithFields(logrus.Fields{"username": username, "email": email}).Debug("Validating registration input")

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
		userId, _ := api.getUserID(db, username)

		if userId != 0 {
			error = "The username is already taken"
		} else {
			// Insert new user into the database
			newUser := User{Username: username, Email: email, PWHash: password}
			err := db.Create(&newUser).Error
			if err != nil {
				logger.WithError(err).Error("Error inserting user")
				log.Println("Error inserting user:", err)
				http.Error(w, "Failed to register user", http.StatusInternalServerError)
				return
			}
		}
	}
	var status int

	if error == "" {
		logger.WithField("username", username).Info("User registered successfully")
		api.metrics.SuccessfulRequests.WithLabelValues("register").Inc()
		w.WriteHeader(http.StatusNoContent)
		status = 200
	} else {
		logger.Warn(error)
		api.metrics.BadRequests.WithLabelValues("register").Inc()
		w.WriteHeader(http.StatusBadRequest)
		status = 400
	}
	response := map[string]interface{}{
		"status":    status,
		"error_msg": error,
	}
	
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Print(err.Error())
		return
	}
}

func (api *API) GETAllMessagesHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)
		
	UpdateLatest(r)

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"query":     r.URL.RawQuery,
		"remote_ip": r.RemoteAddr,
	}).Info("GETAllMessagesHandler called")

	// Retrieve all non-flagged messages
	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = 0").
		Order("messages.message_id DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error

	if err != nil {
		logger.WithError(err).Error("Failed to fetch messages")
		api.metrics.BadRequests.WithLabelValues("get_messages").Inc()
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	logger.WithField("message_count", len(messages)).Info("Messages retrieved successfully")
	api.metrics.SuccessfulRequests.WithLabelValues("msgs").Inc()

	w.Header().Set("Content-Type", "application/json")
	if len(messages) == 0 {
		_, err = w.Write([]byte("[]"))
		if err != nil {
			fmt.Print(err.Error())
		}
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

	err = json.NewEncoder(w).Encode(filteredMsgs)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func (api *API) GETUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)
	
	UpdateLatest(r)

	username := mux.Vars(r)["username"]

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"username":  username,
		"remote_ip": r.RemoteAddr,
	}).Info("GETUserMessagesHandler called")

	// Get user ID
	userID, err := api.getUserID(db, username)
	if err != nil || userID == 0 {
		logger.WithField("username", username).Warn(USER_NOT_FOUND)
		fmt.Printf("Cannot find user: %s", username)
		http.Error(w, "Cannot find user", http.StatusNotFound)
		api.metrics.BadRequests.WithLabelValues("get_user_messages").Inc()
		return
	}

	// Retrieve messages
	var messages []APIMessage
	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON messages.author_id = users.user_id").
		Where("messages.flagged = 0 AND users.user_id = ?", userID).
		Order("messages.message_id DESC").
		Limit(GetNumberHandler(r)).
		Find(&messages).Error

	if err != nil {
		logger.WithError(err).Error("Failed to fetch user messages")
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	api.metrics.SuccessfulRequests.WithLabelValues("get_user_messages").Inc()
	logger.WithField("message_count", len(messages)).Info("User messages retrieved successfully")

	// Ensure empty response is always a valid JSON array
	w.Header().Set("Content-Type", "application/json")
	if len(messages) == 0 {
		_, err = w.Write([]byte("[]"))
		if err != nil {
			fmt.Print(err.Error())
		}
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
	
	err = json.NewEncoder(w).Encode(filteredMsgs)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func (api *API) POSTMessagesHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)

	UpdateLatest(r)

	username := mux.Vars(r)["username"]

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"username":  username,
		"remote_ip": r.RemoteAddr,
	}).Info("POSTMessagesHandler called")

	// Get user ID
	userID, err := api.getUserID(db, username)
	if err != nil || userID == 0 {
		logger.WithField("username", username).Warn(USER_NOT_FOUND)
		fmt.Printf("Cannot find user: %s", username)
		http.Error(w, "Cannot find user", http.StatusNotFound)
		return
	}

	// Read and decode request body
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil || strings.TrimSpace(data.Content) == "" {
		w.Header().Set("Content-Type", "application/json")
		logger.Warn("Invalid or missing content in request")
		http.Error(w, `{"status":400, "error_msg":"Invalid or missing content"}`, http.StatusBadRequest)
		return
	}

	// Create and save message
	message := Message{
		AuthorID: userID,
		Text:     data.Content,
		PubDate:  FormatDateTime(time.Now().Unix()),
		Flagged:  0,
	}

	// Insert into DB
	if err := db.Create(&message).Error; err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.WithError(err).Error("Failed to insert message into database")
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": http.StatusBadRequest,
			"res":    err.Error(),
		})
		if err != nil {
			fmt.Print(err.Error())
		}
		return
	}

	logger.WithField("username", username).Info("Message posted successfully")
	api.metrics.MessagesSent.WithLabelValues("tweet").Inc()

	// Successful response
	w.WriteHeader(http.StatusNoContent)
	api.metrics.SuccessfulRequests.WithLabelValues("tweet").Inc()
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": http.StatusNoContent,
		"res":    "",
	})
	if err != nil {
		fmt.Print(err.Error())
	}
}

func (api *API) GETUserDetailsHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)
	
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"userID":    userID,
		"username":  username,
		"remote_ip": r.RemoteAddr,
	}).Info("GETUserDetailsHandler called")

	var user User
	if userID != "" {
		result := db.Where("user_id = ?", userID).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, USER_NOT_FOUND, http.StatusNotFound)
			} else {
				http.Error(w, "Database error: "+result.Error.Error(), http.StatusInternalServerError)
			}
			return
		}
	} else if username != "" {
		result := db.Where("username = ?", username).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, USER_NOT_FOUND, http.StatusNotFound)
			} else {
				http.Error(w, "Database error: "+result.Error.Error(), http.StatusInternalServerError)
			}
			return
		}
	} else {
		// If neither user_id nor username is provided, return an error
		logger.Warn("Missing user_id or username query parameter")
		api.metrics.BadRequests.WithLabelValues("get_user_details").Inc()
		http.Error(w, "Missing user_id or username query parameter", http.StatusBadRequest)
		return
	}

	userDetails := UserDetails{
		UserID:   user.UserID,
		Username: user.Username,
		Email:    user.Email,
	}

	logger.WithFields(logrus.Fields{"userID": user.UserID, "username": user.Username}).Info("User details retrieved successfully")
	api.metrics.SuccessfulRequests.WithLabelValues("get_user_details").Inc()
	err = json.NewEncoder(w).Encode(userDetails)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func (api *API) GETFollowingHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)

	whoUsername := r.URL.Query().Get("whoUsername")
	whomUsername := r.URL.Query().Get("whomUsername")
	whoUsernameID, _ := api.getUserID(db, whoUsername)
	whomUsernameID, _ := api.getUserID(db, whomUsername)

	logger.WithFields(logrus.Fields{
		"method":       r.Method,
		"path":         r.URL.Path,
		"whoUsername":  whoUsername,
		"whomUsername": whomUsername,
		"remote_ip":    r.RemoteAddr,
	}).Info("GETFollowingHandler called")

	var isFollowing bool = true
	var follower Follower
	err = db.Model(&Follower{}).
		Where("who_id = ? AND whom_id = ?", whoUsernameID, whomUsernameID).
		First(&follower).Error

	if err != nil {
		isFollowing = false // Default to false if no rows found or any error occurs
	}

	logger.WithField("is_following", isFollowing).Info("Following status retrieved successfully")
	api.metrics.SuccessfulRequests.WithLabelValues("get_following").Inc()
	err = json.NewEncoder(w).Encode(isFollowing)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func (api *API) PostLoginHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)

	var req LoginRequest

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"remote_ip": r.RemoteAddr,
	}).Info("PostLoginHandler called")

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.WithError(err).Warn("Invalid request body received")
		api.metrics.BadRequests.WithLabelValues("post_login").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if user exists
	var foundUser User
	result := db.Debug().Where("username = ?", req.Username).First(&foundUser) //db.Where("username = ?", req.Username).First(&foundUser)
	if result.Error == gorm.ErrRecordNotFound {
		logger.WithField("username", req.Username).Warn("Invalid login credentials")
		api.metrics.BadRequests.WithLabelValues("post_login").Inc()
		http.Error(w, "Invalid credentials", http.StatusNotFound)
		return
	} else if result.Error != nil {
		logger.WithError(result.Error).Error("Database error during login")
		api.metrics.BadRequests.WithLabelValues("post_login").Inc()
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// At this point we know that a user exists
	// Check the password hash against the one found in the db
	if req.Password == foundUser.PWHash {
		logger.WithField("username", req.Username).Info("User logged in successfully")
		api.metrics.SuccessfulRequests.WithLabelValues("post_login").Inc()
		w.WriteHeader(http.StatusOK)
	} else {
		logger.WithField("username", req.Username).Warn("Invalid password attempt")
		api.metrics.BadRequests.WithLabelValues("post_login").Inc()
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

}

func (api *API) GetFollowingMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer afterRequestLogging(start, r)
	
	var userID = r.URL.Query().Get("userid")

	logger.WithFields(logrus.Fields{
		"method":    r.Method,
		"path":      r.URL.Path,
		"userID":    userID,
		"remote_ip": r.RemoteAddr,
	}).Info("GetFollowingMessages called")

	var messages []APIMessage

	err = db.Table("messages").
		Select("messages.text AS content, messages.pub_date AS pub_date, users.username AS user").
		Joins("JOIN users ON users.user_id = messages.author_id").
		Where("flagged = ? AND (author_id = ? OR author_id IN (SELECT who_id FROM followers WHERE whom_id = ?))", 0, userID, userID).
		Order("messages.message_id DESC").
		Limit(PER_PAGE).
		Find(&messages).Error

	if err != nil {
		fmt.Println(err.Error())
		logger.WithError(err).Error("Failed to fetch following messages")
		api.metrics.BadRequests.WithLabelValues("get_following_messages").Inc()
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}

	// Convert to Json app Message format

	var filteredMsgs []map[string]string

	for _, msg := range messages {
		filteredMsg := map[string]string{
			"content":  msg.Content,
			"pub_date": msg.PubDate,
			"user":     msg.User,
		}
		filteredMsgs = append(filteredMsgs, filteredMsg)
	}
	api.metrics.SuccessfulRequests.WithLabelValues("get_following_messages").Inc()
	logger.WithField("message_count", len(messages)).Info("Following messages retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(filteredMsgs)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func getPort() {
	port = os.Getenv("PORT")
	if port == "" {
		port = ":9090"
	}
}

func CheckResponse(w http.ResponseWriter, response map[string][]string) {
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func main() {
	initLogger()
	initDB()
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 16, // 16 hours
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	getPort()

	metrics := InitMetrics()      // Initialize metrics
	api := &API{metrics: metrics} // Initialize API with metrics

	// Create a new mux router
	r := mux.NewRouter()

	r.Handle("/metrics", promhttp.Handler())
	// Define the routes and their handlers
	r.HandleFunc("/latest", api.GETLatestHandler).Methods("GET")
	r.HandleFunc("/register", api.RegisterHandler).Methods("POST")
	r.HandleFunc("/fllws/{username}", api.POSTFollowerHandler).Methods("POST")
	r.HandleFunc("/fllws/{username}", api.GETFollowerHandler).Methods("GET")
	r.HandleFunc("/msgs", api.GETAllMessagesHandler).Methods("GET")
	r.HandleFunc("/msgs/{username}", api.GETUserMessagesHandler).Methods("GET")
	r.HandleFunc("/msgs/{username}", api.POSTMessagesHandler).Methods("POST")
	r.HandleFunc("/followingmsgs", api.GetFollowingMessages).Methods("GET")
	r.HandleFunc("/getUserDetails", api.GETUserDetailsHandler).Methods("GET")
	r.HandleFunc("/isfollowing", api.GETFollowingHandler).Methods("GET")
	r.HandleFunc("/login", api.PostLoginHandler).Methods("POST")
	// Start the server on port 7070
	fmt.Printf("Server starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, r))
}

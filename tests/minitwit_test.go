//nolint:govet,errcheck
package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

const baseURL = "http://localhost:8080"

// command to run tests: go test -v (while the app is running)

func createSession() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{Jar: jar}, nil
}

func register(username, password, password2, email string) (*http.Response, error) {
	if password2 == "" {
		password2 = password
	}
	if email == "" {
		email = username + "@example.com"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	data.Set("password2", password2)
	data.Set("email", email)

	return http.PostForm(baseURL+"/register", data)
}

func login(client *http.Client, username, password string) (*http.Response, error) {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	return client.PostForm(baseURL+"/login", data)
}

func logout(client *http.Client) (*http.Response, error) {
	return client.Get(baseURL + "/logout")
}

func addMessage(client *http.Client, text string) (*http.Response, error) {
	data := url.Values{}
	data.Set("text", text)

	return client.PostForm(baseURL+"/add_message", data)
}

// --- TESTS ---
func TestRegister(t *testing.T) {
	resp, _ := register("user1", "default", "", "")

	defer resp.Body.Close()

	assertContains(t, resp, "You were successfully registered and can login now")

	resp, _ = register("user1", "default", "", "")
	defer resp.Body.Close()
	assertContains(t, resp, "The username is already taken")

	resp, _ = register("", "default", "", "")
	defer resp.Body.Close()
	assertContains(t, resp, "You have to enter a username")

	resp, _ = register("meh", "", "", "")
	defer resp.Body.Close()
	assertContains(t, resp, "You have to enter a password")

	resp, _ = register("meh", "x", "y", "")
	defer resp.Body.Close()
	assertContains(t, resp, "The two passwords do not match")

	resp, _ = register("meh", "foo", "", "broken")
	defer resp.Body.Close()
	assertContains(t, resp, "You have to enter a valid email address")
}

func TestLoginLogout(t *testing.T) {
	client, _ := createSession()

	register("user1", "default", "", "")
	resp, _ := login(client, "user1", "default")
	defer resp.Body.Close()
	assertContains(t, resp, "You were logged in")

	resp, _ = logout(client)
	defer resp.Body.Close()
	assertContains(t, resp, "You were logged out")

	resp, _ = login(client, "user1", "wrongpassword")
	defer resp.Body.Close()
	assertContains(t, resp, "Invalid password")

	resp, _ = login(client, "user2", "wrongpassword")
	defer resp.Body.Close()
	assertContains(t, resp, "Invalid username")
}

func TestMessageRecording(t *testing.T) {
	client, _ := createSession()

	register("foo", "default", "", "")
	login(client, "foo", "default")

	addMessage(client, "test message 1")
	addMessage(client, "<test message 2>")

	resp, _ := http.Get(baseURL + "/")
	defer resp.Body.Close()
	assertContains(t, resp, "test message 1")
	assertContains(t, resp, "&lt;test message 2&gt;")
}

func TestTimelines(t *testing.T) {
	clientFoo, _ := createSession()
	register("foo", "default", "", "")
	login(clientFoo, "foo", "default")
	addMessage(clientFoo, "the message by foo")
	logout(clientFoo)

	clientBar, _ := createSession()
	register("bar", "default", "", "")
	login(clientBar, "bar", "default")
	addMessage(clientBar, "the message by bar")

	resp, _ := clientBar.Get(baseURL + "/public_timeline")
	defer resp.Body.Close()
	assertContains(t, resp, "the message by foo")
	assertContains(t, resp, "the message by bar")

	resp, _ = clientBar.Get(baseURL + "/")
	defer resp.Body.Close()
	assertNotContains(t, resp, "the message by foo")
	assertContains(t, resp, "the message by bar")

	resp, _ = clientBar.Get(baseURL + "/foo/follow")
	defer resp.Body.Close()
	assertContains(t, resp, "You are now following foo")

	resp, _ = clientBar.Get(baseURL + "/")
	defer resp.Body.Close()
	assertContains(t, resp, "the message by foo")
	assertContains(t, resp, "the message by bar")

	resp, _ = clientBar.Get(baseURL + "/user_timeline/bar")
	defer resp.Body.Close()
	assertNotContains(t, resp, "the message by foo")
	assertContains(t, resp, "the message by bar")

	resp, _ = clientBar.Get(baseURL + "/user_timeline/foo")
	defer resp.Body.Close()
	assertContains(t, resp, "the message by foo")
	assertNotContains(t, resp, "the message by bar")

	resp, _ = clientBar.Get(baseURL + "/foo/unfollow")
	defer resp.Body.Close()
	assertContains(t, resp, "You are no longer following foo")

	resp, _ = clientBar.Get(baseURL + "/")
	defer resp.Body.Close()
	assertNotContains(t, resp, "the message by foo")
	assertContains(t, resp, "the message by bar")
}

// --- HELPERS ---
func assertContains(t *testing.T, resp *http.Response, expected string) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	body := string(bodyBytes)
	if !strings.Contains(body, expected) {
		t.Errorf("Expected response to contain %q but got %q", expected, body)
	}
}

func assertNotContains(t *testing.T, resp *http.Response, expected string) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	body := string(bodyBytes)
	if strings.Contains(body, expected) {
		t.Errorf("Expected response to contain %q but got %q", expected, body)
	}
}

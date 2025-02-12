package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

const BASE_URL = "http://localhost:8080"

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

	return http.PostForm(BASE_URL+"/register", data)
}

func login(username, password string) (*http.Response, *http.Client, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	resp, err := client.PostForm(BASE_URL+"/login", data)
	return resp, client, err
}

func registerAndLogin(username, password string) (*http.Response, *http.Client, error) {
	register(username, password, "", "")
	return login(username, password)
}

func logout(client *http.Client) (*http.Response, error) {
	return client.Get(BASE_URL + "/logout")
}

func addMessage(client *http.Client, text string) (*http.Response, error) {
	data := url.Values{}
	data.Set("text", text)
	return client.PostForm(BASE_URL+"/add_message", data)
}

// Testing functions
func TestRegister(t *testing.T) {
	resp, _ := register("user65446", "default", "", "")
	defer resp.Body.Close()
	fmt.Println(responseToString(resp))
	if !strings.Contains(responseToString(resp), "You were successfully registered") {
		t.Errorf("Expected successful registration message")
	}
}

func TestLoginLogout(t *testing.T) {
	resp, client, _ := registerAndLogin("user1", "default")
	defer resp.Body.Close()
	if !strings.Contains(responseToString(resp), "You were logged in") {
		t.Errorf("Expected login success message")
	}

	resp, _ = logout(client)
	defer resp.Body.Close()
	if !strings.Contains(responseToString(resp), "You were logged out") {
		t.Errorf("Expected logout success message")
	}
}


func responseToString(resp *http.Response) string {
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

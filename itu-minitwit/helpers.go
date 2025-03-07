package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Taken from https://gowebexamples.com/password-hashing/

func Error() string {
	return "An error occurred."
}

// 
func getUserDetailsByID(w http.ResponseWriter ,userID int, userDetails *UserDetails) error  {
	baseURL := fmt.Sprintf("%s/%s", ENDPOINT,"/getUserDetails")
	u, err := url.Parse(baseURL)
	if err != nil{
		fmt.Print(err.Error())
		http.Error(w,err.Error(),http.StatusBadRequest)
		return err
	}
	
	// Add query parameters
	queryParams := url.Values{}
	//fmt.Print("Remember to change back to userID, %d", userID)
	queryParams.Add("user_id",strconv.Itoa(userID))

	u.RawQuery = queryParams.Encode()
	u.Query()
	res, err := http.Get(u.String())
	if err != nil {
		http.Error(w,err.Error(),http.StatusBadRequest)
		return err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return err
	}
	defer res.Body.Close()
	err = json.Unmarshal(body, &userDetails)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return err
	} 
	return nil
}
func getUserDetailsByUsername(w http.ResponseWriter ,username string, userDetails *UserDetails) error {
	baseURL := fmt.Sprintf("%s/%s", ENDPOINT,"/getUserDetails")
	u, err := url.Parse(baseURL)
	if err != nil{
		fmt.Print(err.Error())
		http.Error(w,err.Error(),http.StatusBadRequest)
		return err 
	}
	
	// Add query parameters
	queryParams := url.Values{}
	queryParams.Add("username",username)
	
	u.RawQuery = queryParams.Encode()
	u.Query()
	res, err := http.Get(u.String())
	if err != nil {
		http.Error(w,err.Error(),http.StatusBadRequest)
		return err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return err
	}
	defer res.Body.Close()
	err = json.Unmarshal(body, &userDetails)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return err
	}
	return nil
}
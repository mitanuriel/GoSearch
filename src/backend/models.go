package main

import (
	"time"
)

type User struct {
	ID              int    `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordChanged bool
}

type PageData struct {
	User         *User
	Error        string
	Title        string
	Template     string
	UserLoggedIn bool
	CSRFToken    string
}

type Page struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Content     string    `json:"content"`
	Language    string    `json:"language"`
	LastUpdated time.Time `json:"last_updated"`
}

type WeatherResponse struct {
	Name string `json:"name"`
	Main struct {
		Temp float64 `json:"temp"`
	} `json:"main"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
}

type PasswordResetData struct {
	UserID            int
	Username          string
	Email             string
	ResetToken        string
	ResetTokenExpires time.Time
	PasswordChanged   bool
}

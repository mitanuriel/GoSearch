package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

// fetchWeatherData calls OpenWeatherMap API to get real weather data
func fetchWeatherData(city string) (*WeatherResponse, error) {
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENWEATHER_API_KEY environment variable not set")
	}

	// OpenWeatherMap free API endpoint
	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather data: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var weatherData WeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherData); err != nil {
		return nil, fmt.Errorf("failed to decode weather data: %w", err)
	}

	return &weatherData, nil
}

// weatherHandler handles HTTP requests for the weather page by fetching current weather for a requested city
// (defaults to "Copenhagen"), preparing a user-facing message, and rendering the weather template.
//
// It reads the session to determine whether a user is logged in, calls fetchWeatherData to obtain weather
// information, and sets the displayed city and message based on the API result. On API error it records a
// failed request metric with the requested city and displays an error message; on success it formats the
// temperature and conditions, records a successful request metric with the API-provided city name, and
// displays the returned city. Template loading or rendering errors are handled via handleInternalError.
func weatherHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"]

	city := r.URL.Query().Get("city")
	if city == "" {
		city = "Copenhagen"
	}

	// Fetch real weather data
	weatherData, err := fetchWeatherData(city)

	var message string
	var displayCity string

	if err != nil {
		// Fallback to mock data if API fails
		log.Printf("Weather API error: %v", err)
		message = fmt.Sprintf("Could not fetch weather data for %s. Please check the city name or try again later.", city)
		displayCity = city
		// Track failed request
		weatherAPIRequests.WithLabelValues(city, "error").Inc()
	} else {
		// Format temperature and description
		temp := fmt.Sprintf("%.1f°C", weatherData.Main.Temp)
		description := "Unknown"
		if len(weatherData.Weather) > 0 {
			description = weatherData.Weather[0].Description
		}
		message = fmt.Sprintf("Temperature: %s, Conditions: %s", temp, description)
		displayCity = weatherData.Name
		// Track successful request with actual city name from API
		weatherAPIRequests.WithLabelValues(displayCity, "success").Inc()
	}

	data := struct {
		Title        string
		City         string
		Message      string
		UserLoggedIn bool
		Template     string
	}{
		Title:        "Weather",
		City:         displayCity,
		Message:      message,
		UserLoggedIn: ok && userID != nil,
		Template:     "weather.html",
	}

	tmpl, err := template.ParseFiles(templatePath+"layout.html", templatePath+"weather.html")
	if err != nil {
		handleInternalError(w, r, err, "Failed to load weather templates")
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		handleInternalError(w, r, err, "Failed to render weather page")
	}
}
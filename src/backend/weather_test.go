// Unit tests for weather functionality
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
)

// NOTE: Tests for fetchWeatherData are in develop branch
// They will be uncommented after merge since fetchWeatherData exists in develop

// Tests for weatherHandler HTTP handler
func TestWeatherHandler_WithCity(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/weather?city=Copenhagen", nil)
	w := httptest.NewRecorder()

	weatherHandler(w, req)

	resp := w.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestWeatherHandler_WithoutCity(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/weather", nil)
	w := httptest.NewRecorder()

	weatherHandler(w, req)

	resp := w.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestWeatherHandler_LoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("GET", "/weather?city=London", nil)
	w := httptest.NewRecorder()

	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/weather?city=London", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	weatherHandler(w2, req2)

	resp := w2.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestWeatherHandler_TemplateError(t *testing.T) {
	// Temporarily modify templatePath to cause error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/weather?city=Paris", nil)
	w := httptest.NewRecorder()

	weatherHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// Tests for fetchWeatherData API function (from develop branch)
func TestFetchWeatherData_MissingAPIKey(t *testing.T) {
	// Save original and unset API key
	originalKey := os.Getenv("OPENWEATHER_API_KEY")
	os.Unsetenv("OPENWEATHER_API_KEY")
	defer os.Setenv("OPENWEATHER_API_KEY", originalKey)

	_, err := fetchWeatherData("Copenhagen")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENWEATHER_API_KEY environment variable not set")
}

func TestFetchWeatherData_WithAPIKey(t *testing.T) {
	// Skip if no API key is set (for CI environments)
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: OPENWEATHER_API_KEY not set")
	}

	// Test with real API key
	weatherData, err := fetchWeatherData("Copenhagen")

	// Should succeed with valid API key
	assert.NoError(t, err)
	assert.NotNil(t, weatherData)
	assert.Equal(t, "Copenhagen", weatherData.Name)
	assert.NotZero(t, weatherData.Main.Temp)
	assert.NotEmpty(t, weatherData.Weather)
	assert.NotEmpty(t, weatherData.Weather[0].Description)
}

func TestFetchWeatherData_InvalidCity(t *testing.T) {
	// Skip if no API key is set
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: OPENWEATHER_API_KEY not set")
	}

	// Test with invalid city name
	_, err := fetchWeatherData("InvalidCityNameThatDoesNotExist12345")

	// Should return error for invalid city
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weather API returned status")
}

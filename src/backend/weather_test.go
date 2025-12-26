package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

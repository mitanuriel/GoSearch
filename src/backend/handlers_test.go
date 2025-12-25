// Unit tests for HTTP handlers (root, about, weather)
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootHandler(t *testing.T) {
	// Initialize templates
	loadTemplates()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	rootHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	
	// Check that the response body is not empty
	assert.Greater(t, w.Body.Len(), 0)
}

func TestAboutHandler(t *testing.T) {
	// Initialize templates
	loadTemplates()

	req := httptest.NewRequest("GET", "/about", nil)
	w := httptest.NewRecorder()

	aboutHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	
	// Check that the response body is not empty
	assert.Greater(t, w.Body.Len(), 0)
}

func TestWeatherHandler(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "Valid city parameter",
			queryParams:    "?city=Copenhagen",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/weather"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			weatherHandler(w, req)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

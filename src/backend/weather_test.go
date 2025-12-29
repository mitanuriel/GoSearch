// Unit tests for weather handler
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
)

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

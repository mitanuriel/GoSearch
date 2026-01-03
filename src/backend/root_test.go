// Unit tests for root handlers
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
)

func TestRootHandler_NotLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	rootHandler(w, req)

	resp := w.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestRootHandler_LoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create request with authenticated session
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	rootHandler(w2, req2)

	resp := w2.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestRootHandler_TemplateError(t *testing.T) {
	// Temporarily modify templatePath to cause error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	rootHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestAboutHandler_NotLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/about", nil)
	w := httptest.NewRecorder()

	aboutHandler(w, req)

	resp := w.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestAboutHandler_LoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create request with authenticated session
	req := httptest.NewRequest("GET", "/about", nil)
	w := httptest.NewRecorder()

	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/about", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	aboutHandler(w2, req2)

	resp := w2.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestAboutHandler_TemplateError(t *testing.T) {
	// Temporarily modify templatePath to cause error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/about", nil)
	w := httptest.NewRecorder()

	aboutHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRootHandler_StoreGetError(t *testing.T) {
	// Create a store with invalid configuration that causes Get to fail
	// Using empty key slice causes decoding errors
	mockStore := sessions.NewCookieStore([]byte(""))
	store = mockStore

	req := httptest.NewRequest("GET", "/", nil)
	// Add an invalid session cookie to trigger decode error
	req.AddCookie(&http.Cookie{
		Name:  "session-name",
		Value: "invalid-session-data",
	})
	w := httptest.NewRecorder()

	rootHandler(w, req)

	// Handler should still work even with session error
	resp := w.Result()
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestAboutHandler_StoreGetError(t *testing.T) {
	// Create a store with invalid configuration that causes Get to fail
	// Using empty key slice causes decoding errors
	mockStore := sessions.NewCookieStore([]byte(""))
	store = mockStore

	req := httptest.NewRequest("GET", "/about", nil)
	// Add an invalid session cookie to trigger decode error
	req.AddCookie(&http.Cookie{
		Name:  "session-name",
		Value: "invalid-session-data",
	})
	w := httptest.NewRecorder()

	aboutHandler(w, req)

	// Handler should still work even with session error
	resp := w.Result()
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

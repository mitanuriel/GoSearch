package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
)

func TestRecordUserRequest_Authenticated(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/search", nil)

	// Record the request
	recordUserRequest(req, "authenticated")

	// Test passes if no panic (metrics are recorded internally)
	// We can't easily verify Prometheus metrics in unit tests, but we ensure no errors
}

func TestRecordUserRequest_Anonymous(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/about", nil)

	// Record the request
	recordUserRequest(req, "anonymous")

	// Test passes if no panic
}

func TestGetAuthStatus_Anonymous(t *testing.T) {
	// Setup session store
	originalStore := store
	store = sessions.NewCookieStore([]byte("test-secret-key"))
	defer func() { store = originalStore }()

	// Create request without session
	req := httptest.NewRequest("GET", "/", nil)

	// Get auth status
	status := getAuthStatus(req)

	// Should return anonymous
	assert.Equal(t, "anonymous", status)
}

func TestGetAuthStatus_Authenticated(t *testing.T) {
	// Setup session store
	originalStore := store
	store = sessions.NewCookieStore([]byte("test-secret-key"))
	defer func() { store = originalStore }()

	// Create request with session
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Create and save session with user_id
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 123
	_ = session.Save(req, w)

	// Create new request with the session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}

	// Get auth status
	status := getAuthStatus(req2)

	// Should return authenticated
	assert.Equal(t, "authenticated", status)
}

func TestGetAuthStatus_NoUserID(t *testing.T) {
	// Setup session store
	originalStore := store
	store = sessions.NewCookieStore([]byte("test-secret-key"))
	defer func() { store = originalStore }()

	// Create request with session but no user_id
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Create and save session WITHOUT user_id
	session, _ := store.Get(req, "session-name")
	session.Values["other_key"] = "value"
	_ = session.Save(req, w)

	// Create new request with the session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}

	// Get auth status
	status := getAuthStatus(req2)

	// Should return anonymous
	assert.Equal(t, "anonymous", status)
}

func TestTrackActiveSession_NewSession(t *testing.T) {
	// Clear active sessions
	activeSessionsLock.Lock()
	activeSessions = make(map[string]bool)
	activeSessionsLock.Unlock()

	// Track a new session
	trackActiveSession("session-123", "authenticated")

	// Verify session is tracked
	activeSessionsLock.Lock()
	exists := activeSessions["session-123"]
	activeSessionsLock.Unlock()

	assert.True(t, exists)
}

func TestTrackActiveSession_ExistingSession(t *testing.T) {
	// Clear and add existing session
	activeSessionsLock.Lock()
	activeSessions = make(map[string]bool)
	activeSessions["session-456"] = true
	activeSessionsLock.Unlock()

	// Track the same session again
	trackActiveSession("session-456", "authenticated")

	// Should still exist (counter not double-incremented)
	activeSessionsLock.Lock()
	exists := activeSessions["session-456"]
	activeSessionsLock.Unlock()

	assert.True(t, exists)
}

func TestRemoveActiveSession_ExistingSession(t *testing.T) {
	// Add a session
	activeSessionsLock.Lock()
	activeSessions = make(map[string]bool)
	activeSessions["session-789"] = true
	activeSessionsLock.Unlock()

	// Remove the session
	removeActiveSession("session-789", "authenticated")

	// Verify session is removed
	activeSessionsLock.Lock()
	exists := activeSessions["session-789"]
	activeSessionsLock.Unlock()

	assert.False(t, exists)
}

func TestRemoveActiveSession_NonExistentSession(t *testing.T) {
	// Clear sessions
	activeSessionsLock.Lock()
	activeSessions = make(map[string]bool)
	activeSessionsLock.Unlock()

	// Try to remove non-existent session (should not panic)
	removeActiveSession("non-existent", "anonymous")

	// Test passes if no panic
}

func TestIncrementUserSessionsTotal(t *testing.T) {
	// Test incrementing authenticated sessions
	incrementUserSessionsTotal("authenticated")

	// Test incrementing anonymous sessions
	incrementUserSessionsTotal("anonymous")

	// Test passes if no panic (Prometheus metrics recorded)
}

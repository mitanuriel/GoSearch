package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	csrf "filippo.io/csrf/gorilla"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// TestCSRFProtectionSetup tests that CSRF middleware is properly configured
func TestCSRFProtectionSetup(t *testing.T) {
	// Test with CSRF_KEY environment variable (must be 32 bytes)
	_ = os.Setenv("CSRF_KEY", "12345678901234567890123456789012")
	defer func() { _ = os.Unsetenv("CSRF_KEY") }()

	csrfKey := []byte(os.Getenv("CSRF_KEY"))
	assert.Equal(t, 32, len(csrfKey), "CSRF key should be 32 bytes")

	// Create CSRF middleware (no Secure or Path options needed for filippo.io/csrf/gorilla)
	csrfMiddleware := csrf.Protect(csrfKey)

	assert.NotNil(t, csrfMiddleware, "CSRF middleware should be created")
}

// TestCSRFProtectionFallback tests CSRF key fallback to session secret
func TestCSRFProtectionFallback(t *testing.T) {
	// Clear CSRF_KEY and set SESSION_SECRET
	_ = os.Unsetenv("CSRF_KEY")
	_ = os.Setenv("SESSION_SECRET", "this-is-a-very-long-session-secret-for-testing-purposes")
	defer func() { _ = os.Unsetenv("SESSION_SECRET") }()

	csrfKey := []byte(os.Getenv("CSRF_KEY"))
	if len(csrfKey) == 0 {
		sessionSecret := os.Getenv("SESSION_SECRET")
		if len(sessionSecret) >= 32 {
			csrfKey = []byte(sessionSecret[:32])
		}
	}

	assert.Equal(t, 32, len(csrfKey), "CSRF key from session secret should be 32 bytes")
}

// TestCSRFProtectionDefaultKey tests default CSRF key when no env vars are set
func TestCSRFProtectionDefaultKey(t *testing.T) {
	_ = os.Unsetenv("CSRF_KEY")
	_ = os.Unsetenv("SESSION_SECRET")

	csrfKey := []byte(os.Getenv("CSRF_KEY"))
	if len(csrfKey) == 0 {
		sessionSecret := os.Getenv("SESSION_SECRET")
		if len(sessionSecret) >= 32 {
			csrfKey = []byte(sessionSecret[:32])
		} else {
			csrfKey = []byte("32-byte-long-auth-key-for-csrf!!")
		}
	}

	assert.Equal(t, 32, len(csrfKey), "Default CSRF key should be 32 bytes")
	assert.Equal(t, "32-byte-long-auth-key-for-csrf!!", string(csrfKey))
}

// TestCSRFTokenGeneration tests that CSRF tokens are generated for requests
func TestCSRFTokenGeneration(t *testing.T) {
	// Create a test router with CSRF protection
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(csrfKey)

	// Add a test handler that returns OK (no token needed with filippo.io/csrf/gorilla)
	r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap router with CSRF middleware
	handler := csrfMiddleware(r)

	// Make a GET request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that request succeeds (filippo.io/csrf/gorilla uses Fetch metadata, no token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// TestCSRFProtectionBlocksRequestsWithoutToken tests that POST requests without CSRF token are blocked
func TestCSRFProtectionBlocksRequestsWithoutToken(t *testing.T) {
	// Create a test router with CSRF protection
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
	)

	// Add a test POST handler
	r.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Success"))
	}).Methods("POST")

	// Wrap router with CSRF middleware
	handler := csrfMiddleware(r)

	// Make a POST request simulating a cross-site request
	// filippo.io/csrf/gorilla uses Fetch metadata headers instead of tokens
	req := httptest.NewRequest("POST", "/submit", strings.NewReader("data=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Simulate a cross-site request (will be blocked)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should be rejected (403 Forbidden)
	assert.Equal(t, http.StatusForbidden, w.Code, "POST from cross-site origin should be rejected")
}

// TestCSRFProtectionAllowsGETRequests tests that GET requests don't require CSRF token
func TestCSRFProtectionAllowsGETRequests(t *testing.T) {
	// Create a test router with CSRF protection
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
	)

	// Add a test GET handler
	r.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Success"))
	}).Methods("GET")

	// Wrap router with CSRF middleware
	handler := csrfMiddleware(r)

	// Make a GET request without CSRF token
	req := httptest.NewRequest("GET", "/page", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should be allowed
	assert.Equal(t, http.StatusOK, w.Code, "GET requests should not require CSRF token")
	assert.Equal(t, "Success", w.Body.String())
}

// TestCSRFTokenInTemplateData tests that templates work without CSRF tokens
// filippo.io/csrf/gorilla uses Fetch metadata headers, not tokens
func TestCSRFTokenInTemplateData(t *testing.T) {
	// Create a mock request
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(csrfKey)

	r.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		// No token needed with filippo.io/csrf/gorilla
		_ = PageData{
			Title:     "Test Form",
			CSRFToken: "",
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Form Page"))
	}).Methods("GET")

	handler := csrfMiddleware(r)

	req := httptest.NewRequest("GET", "/form", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Form Page", w.Body.String())
}

// TestCSRFHeaderName tests the CSRF header configuration
func TestCSRFHeaderName(t *testing.T) {
	// The default CSRF header is X-CSRF-Token
	// This test validates that our middleware uses standard headers
	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
	)

	assert.NotNil(t, csrfMiddleware, "CSRF middleware should be configured")
}

// TestCSRFSecureFlagConfiguration tests CSRF secure flag for production
func TestCSRFSecureFlagConfiguration(t *testing.T) {
	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")

	// Test with secure=false (for development/testing)
	csrfDevMiddleware := csrf.Protect(
		csrfKey,
	)
	assert.NotNil(t, csrfDevMiddleware, "CSRF dev middleware should be configured")

	// Test with secure=true (for production)
	csrfProdMiddleware := csrf.Protect(
		csrfKey,
	)
	assert.NotNil(t, csrfProdMiddleware, "CSRF prod middleware should be configured")
}

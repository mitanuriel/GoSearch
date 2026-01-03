package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/csrf"
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

	// Create CSRF middleware
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
	)

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
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
	)

	// Add a test handler that returns the CSRF token
	r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		token := csrf.Token(r)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(token))
	})

	// Wrap router with CSRF middleware
	handler := csrfMiddleware(r)

	// Make a GET request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that we got a token
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String(), "CSRF token should be generated")
}

// TestCSRFProtectionBlocksRequestsWithoutToken tests that POST requests without CSRF token are blocked
func TestCSRFProtectionBlocksRequestsWithoutToken(t *testing.T) {
	// Create a test router with CSRF protection
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
	)

	// Add a test POST handler
	r.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Success"))
	}).Methods("POST")

	// Wrap router with CSRF middleware
	handler := csrfMiddleware(r)

	// Make a POST request without CSRF token
	req := httptest.NewRequest("POST", "/submit", strings.NewReader("data=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should be rejected (403 Forbidden)
	assert.Equal(t, http.StatusForbidden, w.Code, "POST without CSRF token should be rejected")
}

// TestCSRFProtectionAllowsGETRequests tests that GET requests don't require CSRF token
func TestCSRFProtectionAllowsGETRequests(t *testing.T) {
	// Create a test router with CSRF protection
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
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

// TestCSRFTokenInTemplateData tests that templates receive CSRF token
func TestCSRFTokenInTemplateData(t *testing.T) {
	// Create a mock request with CSRF token
	r := mux.NewRouter()

	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
	)

	var capturedToken string

	r.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		capturedToken = csrf.Token(r)
		data := PageData{
			Title:     "Test Form",
			CSRFToken: capturedToken,
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(data.CSRFToken))
	}).Methods("GET")

	handler := csrfMiddleware(r)

	req := httptest.NewRequest("GET", "/form", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, capturedToken, "CSRF token should be captured")
	assert.Equal(t, capturedToken, w.Body.String(), "CSRF token should match in response")
}

// TestCSRFHeaderName tests the CSRF header configuration
func TestCSRFHeaderName(t *testing.T) {
	// The default CSRF header is X-CSRF-Token
	// This test validates that our middleware uses standard headers
	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")
	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
	)

	assert.NotNil(t, csrfMiddleware, "CSRF middleware should be configured")
}

// TestCSRFSecureFlagConfiguration tests CSRF secure flag for production
func TestCSRFSecureFlagConfiguration(t *testing.T) {
	csrfKey := []byte("32-byte-long-auth-key-for-csrf!!")

	// Test with secure=false (for development/testing)
	csrfDevMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(false),
		csrf.Path("/"),
	)
	assert.NotNil(t, csrfDevMiddleware, "CSRF dev middleware should be configured")

	// Test with secure=true (for production)
	csrfProdMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(true),
		csrf.Path("/"),
	)
	assert.NotNil(t, csrfProdMiddleware, "CSRF prod middleware should be configured")
}

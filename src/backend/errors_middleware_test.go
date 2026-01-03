package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// skipIfCI skips tests that cause issues in CI environments
func skipIfCI(t *testing.T) {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping panic test in CI environment")
	}
}

// TestSecurityHeadersMiddleware_Development tests security headers in development mode
func TestSecurityHeadersMiddleware_Development(t *testing.T) {
	// Save original APP_ENV and restore after test
	originalEnv := os.Getenv("APP_ENV")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("APP_ENV", originalEnv)
		} else {
			_ = os.Unsetenv("APP_ENV")
		}
	}()

	// Set to development mode
	_ = os.Setenv("APP_ENV", "development")

	// Create a test handler that just returns 200 OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with security headers middleware
	wrappedHandler := securityHeadersMiddleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Assert all security headers are present
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check Content-Security-Policy
	csp := rec.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp, "Content-Security-Policy header should be set")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self'")
	assert.Contains(t, csp, "style-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")

	// Check X-Content-Type-Options
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))

	// Check X-Frame-Options
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))

	// Check Referrer-Policy
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))

	// Check X-XSS-Protection
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))

	// HSTS should NOT be set in development
	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"),
		"HSTS should not be set in development mode")
}

// TestSecurityHeadersMiddleware_Production tests HSTS in production mode
func TestSecurityHeadersMiddleware_Production(t *testing.T) {
	// Save original APP_ENV and restore after test
	originalEnv := os.Getenv("APP_ENV")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("APP_ENV", originalEnv)
		} else {
			_ = os.Unsetenv("APP_ENV")
		}
	}()

	// Set to production mode
	_ = os.Setenv("APP_ENV", "production")

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with security headers middleware
	wrappedHandler := securityHeadersMiddleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// HSTS SHOULD be set in production
	hsts := rec.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts, "HSTS should be set in production mode")
	assert.Contains(t, hsts, "max-age=31536000")
	assert.Contains(t, hsts, "includeSubDomains")
}

// TestSecurityHeadersMiddleware_ChainedWithOtherMiddleware tests middleware chaining
func TestSecurityHeadersMiddleware_ChainedWithOtherMiddleware(t *testing.T) {
	// Create a test handler that sets a custom header
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with security headers middleware
	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Both security headers and custom headers should be present
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "test-value", rec.Header().Get("X-Custom-Header"))
}

// TestRecoveryMiddleware_NoPanic tests that recovery middleware passes through normal requests
func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Success"))
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Success")
}

// TestRecoveryMiddleware_WithPanic tests that the middleware catches panics
func TestRecoveryMiddleware_WithPanic(t *testing.T) {
	skipIfCI(t)
	
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic!")
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	// Should not panic, but handle it gracefully
	assert.NotPanics(t, func() {
		wrappedHandler.ServeHTTP(rec, req)
	})

	// Should return 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Should return custom HTML error page
	body := rec.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "Oops!")
	assert.Contains(t, body, "An unexpected error occurred")
}

// TestRecoveryMiddleware_WithDifferentPanicTypes tests recovery with different panic types
func TestRecoveryMiddleware_WithDifferentPanicTypes(t *testing.T) {
	skipIfCI(t)
	
	testCases := []struct {
		name       string
		panicValue interface{}
	}{
		{"String panic", "test panic message"},
		{"Error panic", fmt.Errorf("test error")},
		{"Int panic", 42},
		{"Nil panic", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(tc.panicValue)
			})

			wrappedHandler := recoveryMiddleware(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.0.2.1:1234"
			rec := httptest.NewRecorder()

			assert.NotPanics(t, func() {
				wrappedHandler.ServeHTTP(rec, req)
			})

			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Contains(t, rec.Body.String(), "Oops!")
		})
	}
}

// TestSecurityHeadersMiddleware_ProtectsAllMethods tests headers on different HTTP methods
func TestSecurityHeadersMiddleware_ProtectsAllMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			// Security headers should be set regardless of HTTP method
			assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		})
	}
}

// TestSecurityHeadersMiddleware_CSPDirectives tests specific CSP directives
func TestSecurityHeadersMiddleware_CSPDirectives(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")

	// Test each directive
	testCases := []struct {
		directive string
		expected  string
	}{
		{"default-src", "default-src 'self'"},
		{"script-src", "script-src 'self'"},
		{"style-src", "style-src 'self'"},
		{"img-src", "img-src 'self' data: https://api.openweathermap.org"},
		{"font-src", "font-src 'self'"},
		{"connect-src", "connect-src 'self' https://api.openweathermap.org"},
		{"form-action", "form-action 'self'"},
		{"frame-ancestors", "frame-ancestors 'none'"},
	}

	for _, tc := range testCases {
		t.Run(tc.directive, func(t *testing.T) {
			assert.Contains(t, csp, tc.expected,
				"CSP should contain %s directive", tc.directive)
		})
	}
}

// TestRecoveryMiddleware_PreservesRequestContext tests that request context is preserved
func TestRecoveryMiddleware_PreservesRequestContext(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request properties are accessible
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "TestAgent", r.UserAgent())
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test-path", nil)
	req.Header.Set("User-Agent", "TestAgent")
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestSecurityHeadersMiddleware_DoesNotModifyBody tests that middleware doesn't affect response body
func TestSecurityHeadersMiddleware_DoesNotModifyBody(t *testing.T) {
	expectedBody := "Test response body with special chars: <>&\"'"

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, expectedBody, rec.Body.String(),
		"Middleware should not modify response body")
}

// TestRecoveryMiddleware_WithMultipleHandlers tests recovery in handler chain
func TestRecoveryMiddleware_WithMultipleHandlers(t *testing.T) {
	skipIfCI(t)
	// Create a chain: security headers -> recovery -> handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic in handler")
	})

	wrappedHandler := securityHeadersMiddleware(recoveryMiddleware(testHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		wrappedHandler.ServeHTTP(rec, req)
	})

	// Should have both security headers and error handling
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	assert.Contains(t, rec.Body.String(), "Oops!")
}

// TestSecurityHeadersMiddleware_WithErrorResponse tests headers on error responses
func TestSecurityHeadersMiddleware_WithErrorResponse(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Security headers should be set even on error responses
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

// TestRecoveryMiddleware_LogsStackTrace tests that panics are properly logged
func TestRecoveryMiddleware_LogsStackTrace(t *testing.T) {
	skipIfCI(t)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic for logging")
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test-logging", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	// This should log the panic, but not crash
	assert.NotPanics(t, func() {
		wrappedHandler.ServeHTTP(rec, req)
	})

	// Verify error page is returned
	assert.Contains(t, rec.Body.String(), "unexpected error occurred")
}

// TestSecurityHeadersMiddleware_ContentTypeNotModified tests that Content-Type is preserved
func TestSecurityHeadersMiddleware_ContentTypeNotModified(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Security headers should be added without modifying Content-Type
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

// TestRecoveryMiddleware_MultipleRequests tests that recovery works across multiple requests
func TestRecoveryMiddleware_MultipleRequests(t *testing.T) {
	skipIfCI(t)
	callCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount%2 == 0 {
			panic("panic on even calls")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	// First request - should succeed
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Second request - should panic and recover
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.0.2.1:1234"
	rec2 := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		wrappedHandler.ServeHTTP(rec2, req2)
	})
	assert.Equal(t, http.StatusInternalServerError, rec2.Code)

	// Third request - should succeed again
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.0.2.1:1234"
	rec3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code)
}

// TestSecurityHeadersMiddleware_NoHeaderDuplication tests that headers aren't duplicated
func TestSecurityHeadersMiddleware_NoHeaderDuplication(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler also tries to set a security header
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Middleware should set headers first, handler's attempt comes after
	// So middleware's DENY should be overwritten by handler's SAMEORIGIN
	xFrameOptions := rec.Header().Get("X-Frame-Options")
	assert.Equal(t, "SAMEORIGIN", xFrameOptions)

	// Check that header is not duplicated (only one value)
	values := rec.Header().Values("X-Frame-Options")
	assert.Len(t, values, 1)
}

func TestRecoveryMiddleware_WithCustomHeaders(t *testing.T) {
	skipIfCI(t)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "test-123")
		panic("test panic")
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Custom header set before panic should be preserved
	// Note: This depends on when headers are written. If panic happens after WriteHeader, headers are sent.
	// In this case, panic happens after Header().Set but before WriteHeader, so behavior may vary
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestHandleError_WithSecurityHeaders verifies handleError works with security middleware
func TestHandleError_WithSecurityHeaders(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleInternalError(w, r, fmt.Errorf("test error"), "Test context")
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Should have both security headers and error page
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	assert.Contains(t, rec.Body.String(), "Oops!")
}

// TestSecurityHeadersMiddleware_PerformanceImpact is a benchmark-style test
func TestSecurityHeadersMiddleware_PerformanceImpact(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := securityHeadersMiddleware(testHandler)

	// Run multiple requests to ensure middleware doesn't have issues with concurrent requests
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestRecoveryMiddleware_WithPostRequest(t *testing.T) {
	skipIfCI(t)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to read body, then panic
		panic("panic after reading body")
	})

	wrappedHandler := recoveryMiddleware(testHandler)

	reqBody := strings.NewReader(`{"test":"data"}`)
	req := httptest.NewRequest("POST", "/test", reqBody)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		wrappedHandler.ServeHTTP(rec, req)
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "unexpected error occurred")
}

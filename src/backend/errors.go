package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
)

// ErrorResponse represents a safe error response to send to clients
type ErrorResponse struct {
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

// handleError logs the actual error server-side and sends a generic message to the client
func handleError(w http.ResponseWriter, r *http.Request, err error, statusCode int, userMessage string) {
	// Log detailed error server-side (with request context)
	log.Printf("[ERROR] %s %s - Status: %d - Error: %v - User-Agent: %s - IP: %s",
		r.Method, r.URL.Path, statusCode, err, r.UserAgent(), r.RemoteAddr)

	// Send generic message to client (no internal details exposed)
	http.Error(w, userMessage, statusCode)
}

// handleInternalError is a helper for 500 errors
func handleInternalError(w http.ResponseWriter, r *http.Request, err error, context string) {
	handleError(w, r, err, http.StatusInternalServerError,
		"An unexpected error occurred. Please try again later.")

	// Additional server-side logging with context
	log.Printf("[INTERNAL ERROR] Context: %s - Error: %v", context, err)
}

// handleBadRequest is a helper for 400 errors
func handleBadRequest(w http.ResponseWriter, r *http.Request, userMessage string) {
	log.Printf("[BAD REQUEST] %s %s - Message: %s - User-Agent: %s",
		r.Method, r.URL.Path, userMessage, r.UserAgent())
	http.Error(w, userMessage, http.StatusBadRequest)
}

// recoveryMiddleware catches panics and returns a 500 error instead of crashing
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				log.Printf("[PANIC RECOVERED] %s %s - Panic: %v\nStack Trace:\n%s",
					r.Method, r.URL.Path, err, debug.Stack())

				// Send generic error to client (don't expose panic details)
				http.Error(w, "An unexpected error occurred. Please try again later.",
					http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// logErrorDetails logs error with additional context (for debugging)
func logErrorDetails(context string, err error, details map[string]interface{}) {
	log.Printf("[ERROR DETAILS] Context: %s - Error: %v", context, err)
	if details != nil {
		for key, value := range details {
			log.Printf("  - %s: %v", key, value)
		}
	}
}

// renderErrorPage renders a custom error page (if templates exist)
func renderErrorPage(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	w.WriteHeader(statusCode)
	// For now, just write plain text. You can enhance this to use HTML templates later
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Error %d</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        h1 { color: #e74c3c; }
        p { color: #555; }
        a { color: #3498db; text-decoration: none; }
    </style>
</head>
<body>
    <h1>Oops! Something went wrong</h1>
    <p>%s</p>
    <p><a href="/">Return to homepage</a></p>
</body>
</html>`, statusCode, message)
}

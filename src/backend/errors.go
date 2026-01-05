package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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

	// Send custom HTML error page instead of plain text
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error - GoSearch</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: #333;
        }
        .error-container {
            background: white;
            padding: 3rem;
            border-radius: 15px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            text-align: center;
            max-width: 500px;
        }
        h1 {
            color: #667eea;
            font-size: 2.5rem;
            margin: 0 0 1rem 0;
        }
        p {
            color: #666;
            font-size: 1.1rem;
            margin: 1rem 0;
            line-height: 1.6;
        }
        .home-link {
            display: inline-block;
            margin-top: 2rem;
            padding: 12px 30px;
            background: #667eea;
            color: white;
            text-decoration: none;
            border-radius: 25px;
            transition: all 0.3s ease;
        }
        .home-link:hover {
            background: #764ba2;
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(0,0,0,0.2);
        }
    </style>
</head>
<body>
    <div class="error-container">
        <h1>Oops!</h1>
        <p>%s</p>
        <a href="/" class="home-link">← Back to Home</a>
    </div>
</body>
</html>`, userMessage)
}

// handleInternalError logs additional server-side context for an internal error
// and responds to the client with a generic HTTP 500 error page.
func handleInternalError(w http.ResponseWriter, r *http.Request, err error, context string) {
	// Additional server-side logging with context
	log.Printf("[INTERNAL ERROR] Context: %s - Error: %v", context, err)

	handleError(w, r, err, http.StatusInternalServerError,
		"An unexpected error occurred. Please try again later.")
}

// handleBadRequest is a helper for 400 errors
func handleBadRequest(w http.ResponseWriter, r *http.Request, userMessage string) {
	log.Printf("[BAD REQUEST] %s %s - Message: %s - User-Agent: %s",
		r.Method, r.URL.Path, userMessage, r.UserAgent())
	handleError(w, r, nil, http.StatusBadRequest, userMessage)
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy - prevent XSS and data injection attacks
		// All scripts and styles are now external (no unsafe-inline needed)
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self' data: https://api.openweathermap.org; "+
				"font-src 'self'; "+
				"connect-src 'self' https://api.openweathermap.org; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'")

		// Prevent MIME-sniffing attacks
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		w.Header().Set("X-Frame-Options", "DENY")

		// Control referrer information leakage
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enable XSS protection (for older browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Permissions Policy - restrict browser features to prevent misuse
		// Deny access to camera, microphone, geolocation, and other sensitive features
		w.Header().Set("Permissions-Policy",
			"camera=(), "+
				"microphone=(), "+
				"geolocation=(), "+
				"payment=(), "+
				"usb=(), "+
				"magnetometer=(), "+
				"gyroscope=(), "+
				"accelerometer=()")

		// Only enforce HTTPS in production (not for local development)
		if os.Getenv("APP_ENV") == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware catches panics and returns a 500 error instead of crashing
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				log.Printf("[PANIC RECOVERED] %s %s - Panic: %v\nStack Trace:\n%s",
					r.Method, r.URL.Path, err, debug.Stack())

				// Send custom error page
				handleError(w, r, fmt.Errorf("%v", err), http.StatusInternalServerError,
					"An unexpected error occurred. Please try again later.")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// Commented out unused helper functions - uncomment when needed in future

// logErrorDetails logs error with additional context (for debugging)
// func logErrorDetails(context string, err error, details map[string]interface{}) {
// 	log.Printf("[ERROR DETAILS] Context: %s - Error: %v", context, err)
// 	for key, value := range details {
// 		log.Printf("  - %s: %v", key, value)
// 	}
// }

// renderErrorPage renders a custom error page (if templates exist)
// func renderErrorPage(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
// 	w.WriteHeader(statusCode)
// 	fmt.Fprintf(w, `<!DOCTYPE html>
// <html>
// <head>
//     <title>Error %d</title>
//     <style>
//         body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
//         h1 { color: #e74c3c; }
//         p { color: #555; }
//         a { color: #3498db; text-decoration: none; }
//     </style>
// </head>
// <body>
//     <h1>Oops! Something went wrong</h1>
//     <p>%s</p>
//     <p><a href="/">Return to homepage</a></p>
// </body>
// </html>`, statusCode, message)
// }
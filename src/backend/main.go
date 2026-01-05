package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	csrf "filippo.io/csrf/gorilla"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// main initializes application services, configures HTTP routes and middleware, and starts the web server.
//
// It performs startup tasks including database initialization and readiness checks, password-reset table
// setup, Elasticsearch initialization and page synchronization, search log setup, table checks and cron
// scheduler startup, and optional scraping. It configures monitoring and metrics, registers routes and
// middleware (security headers, recovery, password reset, CSRF protection, and metrics), serves static
// files, exposes a health check and Prometheus metrics endpoint, and listens on :8080.
func main() {

	log.Printf("CONN_STR: %s", CONN_STR)
	// initialiserer databasen og forbinder til den.
	initDB()
	defer closeDB()

	// Wait for database to be fully ready
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		if err := db.Ping(); err != nil {
			log.Printf("Database not ready yet, retrying in 2 seconds... (%d/%d)", i+1, maxRetries)
			time.Sleep(2 * time.Second)
		} else {
			log.Println("Database connection confirmed!")
			break
		}

		if i == maxRetries-1 {
			log.Fatalf("Failed to connect to database after %d attempts", maxRetries)
		}
	}

	err := setupPasswordResetTable()
	if err != nil {
		log.Printf("Warning: Password reset setup had errors: %v", err)
		log.Println("Will attempt to continue startup anyway...")
	} else {
		log.Println("Password reset functionality successfully initialized")
	}

	//!!!Only comment in if all passwords of all users needs to be reset!!!

	/*if err := forceResetForAllUsers(); err != nil {
		log.Printf("Warning: Failed to force password reset for all users: %v", err)
	} else {
		log.Println("Successfully forced all users to reset their passwords")
	}*/

	//Initialize Elasticsearch
	initElasticsearch()

	if err := syncPagesToElasticsearch(); err != nil {
		log.Fatalf("Failed to sync pages: %v", err)
	}

	logPath := os.Getenv("SEARCH_LOG_PATH")
	if logPath == "" {
		logPath = "search.log" // Default for Docker
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: could not open search log file: %v, using stdout instead", err)
		searchLogger = log.New(os.Stdout, "SEARCH: ", log.LstdFlags)
	} else {
		log.Printf("Search logs will be written to %s", logPath)
		searchLogger = log.New(f, "SEARCH: ", log.LstdFlags)
		defer func() { _ = f.Close() }()
	}

	// Run checkTables once at startup, then start the cron scheduler for periodic checks
	checkTables()
	startCronScheduler()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	fmt.Println("Database connection successful!")

	startMonitoring()

	//Scraper hvis ønsket - hvis miljø variabel er sat til 1.
	if os.Getenv("SCRAPING_ENABLED") == "1" {
		StartScraping(logPath)
	}

	// Detter er Gorilla Mux's route handler, i stedet for Flasks indbyggede router-handler
	///Opretter en ny router
	r := mux.NewRouter()

	// Add security headers middleware first (applies to all routes)
	r.Use(securityHeadersMiddleware)
	// Add recovery middleware to catch panics
	r.Use(recoveryMiddleware)
	r.Use(passwordResetMiddleware)

	// Setup CSRF protection using Fetch metadata headers
	// filippo.io/csrf/gorilla uses modern browser security features (Sec-Fetch-Site header)
	// instead of tokens, making it more secure and simpler
	csrfKeyStr := os.Getenv("CSRF_KEY")
	csrfKey := []byte(csrfKeyStr)
	if len(csrfKey) != 32 {
		// CSRF key must be exactly 32 bytes - use session secret as fallback
		sessionSecret := os.Getenv("SESSION_SECRET")
		if len(sessionSecret) >= 32 {
			csrfKey = []byte(sessionSecret[:32])
		} else {
			// Default fallback key (exactly 32 bytes)
			csrfKey = []byte("32-byte-long-auth-key-for-csrf!!")
		}
		log.Printf("CSRF key invalid or missing (got %d bytes), using fallback (32 bytes)", len(csrfKeyStr))
	}

	// Note: filippo.io/csrf/gorilla does not use Secure() or Path() options
	// It relies on Fetch metadata headers which are more secure
	csrfMiddleware := csrf.Protect(csrfKey)

	fmt.Println("Registering /metrics endpoint...")
	r.Handle("/metrics", promhttp.Handler())

	// Applying middleware function to all routes
	appRouter := r.NewRoute().Subrouter()
	appRouter.Use(metricsMiddleware)

	//Definerer routerne.
	appRouter.HandleFunc("/", rootHandler).Methods("GET")             // Forside
	appRouter.HandleFunc("/about", aboutHandler).Methods("GET")       //about-side
	appRouter.HandleFunc("/login", login).Methods("GET")              //Login-side
	appRouter.HandleFunc("/register", registerHandler).Methods("GET") //Register-side
	appRouter.HandleFunc("/search", searchHandler).Methods("GET")
	appRouter.HandleFunc("/reset-password", resetPasswordHandler).Methods("GET")

	// Health check endpoint (no CSRF, no session required)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}).Methods("GET")

	// Definerer api-erne
	appRouter.HandleFunc("/api/login", apiLogin).Methods("POST")
	appRouter.HandleFunc("/api/logout", logoutHandler).Methods("GET")
	appRouter.HandleFunc("/api/search", searchHandler).Methods("GET")
	appRouter.HandleFunc("/api/search", searchHandler).Methods("POST") // API-ruten for søgninger.
	appRouter.HandleFunc("/api/register", apiRegisterHandler).Methods("POST")
	appRouter.HandleFunc("/api/weather", weatherHandler).Methods("GET") //weather-side
	appRouter.HandleFunc("/api/reset-password", apiResetPasswordHandler).Methods("POST")

	// sørger for at vi kan bruge de statiske filer som ligger i static-mappen. ex: css.
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	fmt.Println("Registering /metrics endpoint...")
	r.Handle("/metrics", promhttp.Handler())

	fmt.Println("Server running on http://localhost:8080")
	//Starter serveren.
	log.Fatal(http.ListenAndServe(":8080", csrfMiddleware(r)))

}
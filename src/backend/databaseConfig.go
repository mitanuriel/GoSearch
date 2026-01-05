package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

func connectDB() (*sql.DB, error) {
	var db *sql.DB
	var err error

	maxRetries := 10
	retryDelay := time.Second * 5

	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", CONN_STR)
		if err != nil {
			log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryDelay)
			continue
		}
		err = db.Ping()
		if err == nil {
			log.Println("Successfully connected to PostgresSQL!")
			return db, nil
		}
		log.Printf("Database ping failed (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("failed to connect to database after %d attempts", maxRetries)

}

func initDB() {
	var err error
	db, err = connectDB()
	if err != nil {
		log.Fatalf("%v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("PostgresSQL ping failed: %v", err)
	}
	log.Println("Connected to PostgresSQL!")

}

func queryDB(query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(query, args...)
}

func closeDB() {
	if db != nil {
		_ = db.Close()
	}
}
func checkTables() {
	// Check users table
	fmt.Println("\n--- Users in database ---")
	rows, err := queryDB("SELECT * FROM users")
	if err != nil {
		log.Printf("Error querying users: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.PasswordChanged)
		if err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}
		fmt.Printf("ID: %d, Username: %s, Email: %s, Password Changed: %t\n", user.ID, user.Username, user.Email, user.PasswordChanged)
	}

	// Check pages table
	fmt.Println("\n--- Pages in database ---")
	rows2, err := queryDB("SELECT * FROM pages")
	if err != nil {
		log.Printf("Error querying pages: %v", err)
		return
	}
	defer func() { _ = rows2.Close() }()

	for rows2.Next() {
		var page Page
		err := rows2.Scan(&page.Title, &page.URL, &page.Language, &page.LastUpdated, &page.Content)
		if err != nil {
			log.Printf("Error scanning page: %v", err)
			continue
		}
		fmt.Printf("Title: %s, URL: %s, Language: %s\n", page.Title, page.URL, page.Language)
	}
}

// runCheckTables is a cron job handler for checking database tables
func runCheckTables() {
	fmt.Println("Cron job: Running checkTables at", time.Now())
	checkTables()
}

// runDatabaseBackup is a cron job handler for database backup and cleanup
func runDatabaseBackup() {
	log.Println("Cron job: Running database backup at", time.Now())
	backupDatabase()
	cleanupOldBackups()
}

// runWikipediaScraper is a cron job handler for Wikipedia scraping
func runWikipediaScraper() {
	fmt.Println("Cron job: Running Wikipedia scraper at", time.Now())
	logPath := os.Getenv("SEARCH_LOG_PATH")
	if logPath == "" {
		logPath = "search.log"
	}

	// Track the number of pages before scraping
	var countBefore int
	err := db.QueryRow("SELECT COUNT(*) FROM pages").Scan(&countBefore)
	if err != nil {
		log.Printf("Error getting page count before scraping: %v", err)
	}

	// Run scraping
	StartScraping(logPath)

	// Check if new pages were added
	var countAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM pages").Scan(&countAfter)
	if err != nil {
		log.Printf("Error getting page count after scraping: %v", err)
	}

	// Only sync to Elasticsearch if new pages were added
	if countAfter > countBefore {
		log.Printf("New pages added (%d -> %d). Syncing to Elasticsearch.", countBefore, countAfter)
		err := syncPagesToElasticsearch()
		if err != nil {
			log.Printf("Error syncing to Elasticsearch: %v", err)
		} else {
			log.Println("Synced scraped pages to Elasticsearch successfully.")
		}
	} else {
		log.Println("No new pages added. Skipping Elasticsearch sync.")
	}
}

func startCronScheduler() {
	c := cron.New()

	// Schedule the checkTables function to run every minute
	if _, err := c.AddFunc("*/1 * * * *", runCheckTables); err != nil {
		log.Fatalf("Error scheduling cron job: %v", err)
	}

	// Schedule database backup to run daily at 2 AM
	if _, err := c.AddFunc("0 2 * * *", runDatabaseBackup); err != nil {
		log.Fatalf("Error scheduling backupDatabase cron job: %v", err)
	}

	// Schedule Wikipedia scraping every 5 minutes
	if _, err := c.AddFunc("*/5 * * * *", runWikipediaScraper); err != nil {
		log.Fatalf("Error scheduling Wikipedia scraper cron job: %v", err)
	}

	c.Start()
}

// parseConnectionString parses a PostgreSQL connection string in either URL form
// (postgres://user:pass@host:port/dbname) or key=value form (host=... port=... user=... password=... dbname=...).
// It returns host, port (defaults to "5432" when missing), user, password, dbname, and a non-nil error if host or dbname are missing or the string is invalid.
func parseConnectionString(connStr string) (host, port, user, password, dbname string, err error) {
	// Try parsing the URL format first
	if connURL, parseErr := url.Parse(connStr); parseErr == nil && connURL.Scheme == "postgres" {
		// Format: postgres://username:password@host:port/dbname
		host = connURL.Hostname()
		port = connURL.Port()
		if port == "" {
			port = "5432" // Default PostgreSQL port
		}
		user = connURL.User.Username()
		password, _ = connURL.User.Password()
		dbname = strings.TrimPrefix(connURL.Path, "/")

		// Validate that we got the essential fields
		if host == "" || dbname == "" {
			return "", "", "", "", "", fmt.Errorf("invalid postgres URL: missing host or database name")
		}
		return host, port, user, password, dbname, nil
	}

	// Format: host=localhost port=5432 user=postgres password=secret dbname=mydb
	params := make(map[string]string)
	parts := strings.Fields(connStr)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[strings.ToLower(kv[0])] = kv[1]
		}
	}

	host = params["host"]
	port = params["port"]
	if port == "" {
		port = "5432"
	}
	user = params["user"]
	password = params["password"]
	dbname = params["dbname"]

	// Validate that we got the essential fields
	if host == "" || dbname == "" {
		return "", "", "", "", "", fmt.Errorf("invalid connection string: missing host or dbname")
	}

	return host, port, user, password, dbname, nil
}

// executePgDump runs pg_dump command and creates backup file
func executePgDump(host, port, user, password, dbname, outputFile string) error {
	// Check if pg_dump is available
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return fmt.Errorf("pg_dump not found in PATH: %w", err)
	}
	log.Printf("Using pg_dump from: %s", pgDumpPath)

	// Create the pg_dump command
	cmd := exec.Command(pgDumpPath,
		"-h", host,
		"-p", port,
		"-U", user,
		"-F", "c", // Custom format
		"-b", // Include large objects
		"-v", // Verbose
		"-f", outputFile,
		dbname)

	// Set PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	// Run the command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("backup failed: %v\nCommand output: %s", err, string(output))
	}

	return nil
}

// verifyBackupFile checks if backup file was created and reports its size
func verifyBackupFile(outputFile string) error {
	// Check if the file was actually created
	fileInfo, err := os.Stat(outputFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("backup file not created: %s", outputFile)
	}
	if err != nil {
		return fmt.Errorf("error getting backup file info: %w", err)
	}

	if fileInfo.Size() == 0 {
		log.Printf("Warning: Backup file is empty: %s", outputFile)
	} else {
		log.Printf("Backup successful: %s (%.2f MB)", outputFile, float64(fileInfo.Size())/1024/1024)
	}

	return nil
}

func backupDatabase() {
	// Create backups directory if it doesn't exist
	backupDir := "/app/src/backend/backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Printf("Failed to create backup directory: %v", err)
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	outputFile := filepath.Join(backupDir, fmt.Sprintf("backup_%s.sql", timestamp))

	log.Printf("Using connection string: %s", CONN_STR)

	// Parse connection string
	dbHost, dbPort, dbUser, dbPassword, dbName, err := parseConnectionString(CONN_STR)
	if err != nil || dbHost == "" || dbUser == "" || dbName == "" {
		log.Printf("Backup failed: Couldn't extract required database parameters from connection string")
		log.Printf("Host: %s, User: %s, DB Name: %s", dbHost, dbUser, dbName)
		return
	}

	log.Printf("Extracted database parameters - Host: %s, Port: %s, User: %s, DB: %s",
		dbHost, dbPort, dbUser, dbName)

	// Execute pg_dump
	if err := executePgDump(dbHost, dbPort, dbUser, dbPassword, dbName, outputFile); err != nil {
		log.Printf("%v", err)
		return
	}

	// Verify backup file was created successfully
	if err := verifyBackupFile(outputFile); err != nil {
		log.Printf("%v", err)
	}
}

func cleanupOldBackups() {
	dir := "/app/src/backend/backups"

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Failed to read backup directory: %v", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -7) // 7 days ago
	var totalRemoved int

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Printf("Failed to get file info for %s: %v", entry.Name(), err)
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(dir, entry.Name())
			err := os.Remove(fullPath)
			if err != nil {
				log.Printf("Failed to delete old backup %s: %v", fullPath, err)
			} else {
				log.Printf("Deleted old backup: %s (%.2f MB)", fullPath, float64(info.Size())/1024/1024)
				totalRemoved++
			}
		}
	}

	if totalRemoved > 0 {
		log.Printf("Cleanup complete: Removed %d old backup files", totalRemoved)
	} else {
		log.Printf("Cleanup complete: No old backups found to remove")
	}
}

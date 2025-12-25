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
		err := rows2.Scan(&page.URL, &page.Title, &page.Language, &page.LastUpdated, &page.Content)
		if err != nil {
			log.Printf("Error scanning page: %v", err)
			continue
		}
		fmt.Printf("Title: %s, URL: %s, Language: %s\n", page.Title, page.URL, page.Language)
	}
}

func startCronScheduler() {
	c := cron.New()
	// Schedule the checkTables function to run every minute
	if _, err := c.AddFunc("*/1 * * * *", func() {
		fmt.Println("Cron job: Running checkTables at", time.Now())
		checkTables()
	}); err != nil {
		log.Fatalf("Error scheduling cron job: %v", err)
	}

	if _, err := c.AddFunc("0 2 * * *", func() {
		log.Println("Cron job: Running database backup at", time.Now())
		backupDatabase()
		cleanupOldBackups()
	}); err != nil {
		log.Fatalf("Error scheduling backupDatabase cron job: %v", err)
	}

	// scraping wikipedia every 5. minutes
	if _, err := c.AddFunc("*/5 * * * *", func() {
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
	}); err != nil {
		log.Fatalf("Error scheduling Wikipedia scraper cron job: %v", err)
	}

	c.Start()
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

	// Use the connection string that's already been loaded in config.go
	// We need to parse it to extract the individual pieces for pg_dump

	log.Printf("Using connection string: %s", CONN_STR)

	var dbHost, dbPort, dbUser, dbName, dbPassword string

	// Try parsing the URL format
	if connURL, err := url.Parse(CONN_STR); err == nil && connURL.Scheme == "postgres" {
		// Format: postgres://username:password@host:port/dbname
		dbHost = connURL.Hostname()
		dbPort = connURL.Port()
		if dbPort == "" {
			dbPort = "5432" // Default PostgreSQL port
		}
		dbUser = connURL.User.Username()
		dbPassword, _ = connURL.User.Password()
		dbName = strings.TrimPrefix(connURL.Path, "/")
	} else {
		// Format: host=localhost port=5432 user=postgres password=secret dbname=mydb
		params := make(map[string]string)
		parts := strings.Fields(CONN_STR)
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				params[strings.ToLower(kv[0])] = kv[1]
			}
		}

		dbHost = params["host"]
		dbPort = params["port"]
		if dbPort == "" {
			dbPort = "5432" // Default PostgreSQL port
		}
		dbUser = params["user"]
		dbPassword = params["password"]
		dbName = params["dbname"]
	}

	// Validate that we have the required connection parameters
	if dbHost == "" || dbUser == "" || dbName == "" {
		log.Printf("Backup failed: Couldn't extract required database parameters from connection string")
		log.Printf("Host: %s, User: %s, DB Name: %s", dbHost, dbUser, dbName)
		return
	}

	log.Printf("Extracted database parameters - Host: %s, Port: %s, User: %s, DB: %s",
		dbHost, dbPort, dbUser, dbName)

	// Check if pg_dump is available
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		log.Printf("pg_dump not found in PATH: %v", err)
		log.Printf("Please install PostgreSQL client tools")
		return
	}
	log.Printf("Using pg_dump from: %s", pgDumpPath)

	// Create the pg_dump command
	cmd := exec.Command(pgDumpPath,
		"-h", dbHost,
		"-p", dbPort,
		"-U", dbUser,
		"-F", "c", // Custom format
		"-b", // Include large objects
		"-v", // Verbose
		"-f", outputFile,
		dbName)

	// Set PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), "PGPASSWORD="+dbPassword)

	// Run the command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Backup failed: %v\nCommand output: %s", err, string(output))
		return
	}

	// Check if the file was actually created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		log.Printf("Backup command succeeded but output file not created: %s", outputFile)
		log.Printf("Command output: %s", string(output))
		return
	}

	// Get file size
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		log.Printf("Error getting backup file info: %v", err)
	} else if fileInfo.Size() == 0 {
		log.Printf("Warning: Backup file is empty: %s", outputFile)
	} else {
		log.Printf("Backup successful: %s (%.2f MB)", outputFile, float64(fileInfo.Size())/1024/1024)
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

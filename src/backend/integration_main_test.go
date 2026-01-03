// +build integration

// Integration tests for main server startup and infrastructure
// Run with: go test -tags=integration ./src/backend/...
package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConnectDB tests database connection with retry logic
func TestConnectDB_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Save original CONN_STR
	originalConnStr := CONN_STR
	defer func() { CONN_STR = originalConnStr }()

	t.Run("Successful connection", func(t *testing.T) {
		// Use real connection string from environment
		connStr := os.Getenv("CONN_STR")
		if connStr == "" {
			t.Skip("CONN_STR not set, skipping test")
		}
		CONN_STR = connStr

		testDB, err := connectDB()
		assert.NoError(t, err)
		assert.NotNil(t, testDB)

		if testDB != nil {
			err = testDB.Ping()
			assert.NoError(t, err)
			testDB.Close()
		}
	})

	t.Run("Failed connection with invalid connection string", func(t *testing.T) {
		// Set invalid connection string
		CONN_STR = "postgres://invalid:invalid@localhost:5432/invalid?sslmode=disable"

		testDB, err := connectDB()
		// Should fail after retries
		assert.Error(t, err)
		assert.Nil(t, testDB)
	})
}

// TestInitDB tests database initialization
func TestInitDB_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Save original db and CONN_STR
	originalDB := db
	originalConnStr := CONN_STR
	defer func() {
		db = originalDB
		CONN_STR = originalConnStr
	}()

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}
	CONN_STR = connStr

	// Reset db to nil to test initialization
	db = nil

	// This should not panic or fail
	initDB()

	assert.NotNil(t, db)
	err := db.Ping()
	assert.NoError(t, err)
}

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if server is running (this assumes the server is started separately)
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		t.Skipf("Server not running at %s, skipping test: %v", serverURL, err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestDatabaseOperations tests basic database operations
func TestDatabaseOperations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	testDB, err := sql.Open("postgres", connStr)
	assert.NoError(t, err)
	defer testDB.Close()

	err = testDB.Ping()
	assert.NoError(t, err)

	// Test simple query
	var result int
	err = testDB.QueryRow("SELECT 1").Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	// Test table existence checks
	var exists bool
	err = testDB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'users'
		)
	`).Scan(&exists)
	assert.NoError(t, err)
	assert.True(t, exists, "users table should exist")

	err = testDB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'pages'
		)
	`).Scan(&exists)
	assert.NoError(t, err)
	assert.True(t, exists, "pages table should exist")
}

// TestSetupPasswordResetTable_Integration tests password reset table setup
func TestSetupPasswordResetTable_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Save original db
	originalDB := db
	defer func() { db = originalDB }()

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	var err error
	db, err = sql.Open("postgres", connStr)
	assert.NoError(t, err)
	defer db.Close()

	err = db.Ping()
	assert.NoError(t, err)

	// Run setupPasswordResetTable
	err = setupPasswordResetTable()
	assert.NoError(t, err)

	// Verify password_changed column exists
	var columnExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_name = 'users' 
			AND column_name = 'password_changed'
		)
	`).Scan(&columnExists)
	assert.NoError(t, err)
	assert.True(t, columnExists, "password_changed column should exist")

	// Verify reset_tokens table exists
	var tableExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'reset_tokens'
		)
	`).Scan(&tableExists)
	assert.NoError(t, err)
	assert.True(t, tableExists, "reset_tokens table should exist")
}

// TestVerifySetup_Integration tests the setup verification
func TestVerifySetup_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Save original db
	originalDB := db
	defer func() { db = originalDB }()

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	var err error
	db, err = sql.Open("postgres", connStr)
	assert.NoError(t, err)
	defer db.Close()

	err = db.Ping()
	assert.NoError(t, err)

	// First ensure setup is complete
	err = setupPasswordResetTable()
	assert.NoError(t, err)

	// Now verify the setup
	err = verifySetup()
	assert.NoError(t, err)
}

// TestMetricsEndpoint tests the /metrics endpoint
func TestMetricsEndpoint_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(serverURL + "/metrics")
	if err != nil {
		t.Skipf("Server not running at %s, skipping test: %v", serverURL, err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
}

// TestStaticFiles tests static file serving
func TestStaticFiles_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Test CSS file
	resp, err := client.Get(serverURL + "/static/style.css")
	if err != nil {
		t.Skipf("Server not running at %s, skipping test: %v", serverURL, err)
	}
	defer resp.Body.Close()

	// Should return 200 or 404 (depending on whether file exists)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound,
		fmt.Sprintf("Expected 200 or 404, got %d", resp.StatusCode))
}

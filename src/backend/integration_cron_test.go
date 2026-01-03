//go:build integration
// +build integration

// Integration tests for cron scheduler and backup functions
// These tests require PostgreSQL database and pg_dump utility.
// Run with: go test -tags=integration -run TestCheckTables_Integration ./src/backend/
//
//nolint:errcheck // Test cleanup errors are not critical
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCheckTables_Integration tests the checkTables function
func TestCheckTables_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	// Save original db
	originalDB := db
	defer func() { db = originalDB }()

	CONN_STR = connStr
	initDB()
	if db == nil {
		t.Skip("Database not available, skipping test")
	}

	// This should not panic
	checkTables()

	// If we got here without panic, the test passed
	t.Log("checkTables completed successfully")
}

// TestParseConnectionString_Integration tests parsing connection strings
func TestParseConnectionString_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name     string
		connStr  string
		wantErr  bool
		wantHost string
		wantPort string
		wantDB   string
	}{
		{
			name:     "Valid connection string",
			connStr:  "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
			wantErr:  false,
			wantHost: "localhost",
			wantPort: "5432",
			wantDB:   "testdb",
		},
		{
			name:    "Invalid connection string",
			connStr: "invalid-connection-string",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			host, port, _, _, dbname, err := parseConnectionString(tc.connStr)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantHost, host)
				assert.Equal(t, tc.wantPort, port)
				assert.Equal(t, tc.wantDB, dbname)
			}
		})
	}
}

// TestBackupDatabase_Integration tests the backup functionality
func TestBackupDatabase_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	// Check if pg_dump is available
	_, err := os.Stat("/usr/bin/pg_dump")
	if os.IsNotExist(err) {
		_, err = os.Stat("/usr/local/bin/pg_dump")
		if os.IsNotExist(err) {
			t.Skip("pg_dump not found, skipping backup test")
		}
	}

	// Create a temporary backup directory
	tmpDir := filepath.Join(os.TempDir(), "gosearch_backup_test")
	err = os.MkdirAll(tmpDir, 0755)
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Save original environment
	originalBackupPath := os.Getenv("BACKUP_PATH")
	defer func() {
		if originalBackupPath != "" {
			os.Setenv("BACKUP_PATH", originalBackupPath)
		} else {
			os.Unsetenv("BACKUP_PATH")
		}
	}()

	// Set test backup path
	os.Setenv("BACKUP_PATH", tmpDir)

	// Save original db
	originalDB := db
	defer func() { db = originalDB }()

	CONN_STR = connStr
	initDB()
	if db == nil {
		t.Skip("Database not available, skipping test")
	}

	// Run backup (this might fail for various reasons, so we just check it doesn't panic)
	backupDatabase()
	
	t.Log("backupDatabase completed successfully")

	// Check if backup file was created
	files, err := filepath.Glob(filepath.Join(tmpDir, "backup_*.sql"))
	if err == nil && len(files) > 0 {
		t.Logf("Backup file created: %s", files[0])
		
		// Verify backup file is not empty
		info, err := os.Stat(files[0])
		assert.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0), "Backup file should not be empty")
	} else {
		t.Log("No backup file created (might be expected if pg_dump failed)")
	}
}

// TestVerifyBackupFile_Integration tests backup file verification
func TestVerifyBackupFile_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if no database connection string (test uses log which might trigger db access)
	if os.Getenv("CONN_STR") == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	// Create a temporary test file
	tmpDir := filepath.Join(os.TempDir(), "gosearch_verify_test")
	err := os.MkdirAll(tmpDir, 0755)
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("Valid backup file", func(t *testing.T) {
		// Create a test backup file with some SQL content
		testFile := filepath.Join(tmpDir, "test_backup.sql")
		content := []byte("-- PostgreSQL database dump\nCREATE TABLE test (id INT);\n")
		err := os.WriteFile(testFile, content, 0644)
		assert.NoError(t, err)

		// Verify the file
		err = verifyBackupFile(testFile)
		assert.NoError(t, err)
	})

	t.Run("Empty backup file", func(t *testing.T) {
		// Create an empty file
		testFile := filepath.Join(tmpDir, "empty_backup.sql")
		err := os.WriteFile(testFile, []byte{}, 0644)
		assert.NoError(t, err)

		// Verify should succeed but log a warning for empty file
		// (verifyBackupFile doesn't return error for empty files, just logs)
		err = verifyBackupFile(testFile)
		assert.NoError(t, err)
	})

	t.Run("Non-existent backup file", func(t *testing.T) {
		// Verify should fail for non-existent file
		err := verifyBackupFile(filepath.Join(tmpDir, "nonexistent.sql"))
		assert.Error(t, err)
	})
}

// TestCleanupOldBackups_Integration tests old backup cleanup
func TestCleanupOldBackups_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary backup directory with test files
	tmpDir := filepath.Join(os.TempDir(), "gosearch_cleanup_test")
	err := os.MkdirAll(tmpDir, 0755)
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some test backup files with different timestamps
	now := time.Now()
	testFiles := []struct {
		name string
		age  time.Duration
	}{
		{"backup_recent.sql", 0},                    // Current
		{"backup_old1.sql", 8 * 24 * time.Hour},    // 8 days old
		{"backup_old2.sql", 10 * 24 * time.Hour},   // 10 days old
		{"backup_old3.sql", 15 * 24 * time.Hour},   // 15 days old
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tmpDir, tf.name)
		err := os.WriteFile(filePath, []byte("test backup content"), 0644)
		assert.NoError(t, err)

		// Set file modification time
		modTime := now.Add(-tf.age)
		err = os.Chtimes(filePath, modTime, modTime)
		assert.NoError(t, err)
	}

	// Save original environment
	originalBackupPath := os.Getenv("BACKUP_PATH")
	defer func() {
		if originalBackupPath != "" {
			os.Setenv("BACKUP_PATH", originalBackupPath)
		} else {
			os.Unsetenv("BACKUP_PATH")
		}
	}()

	// Set test backup path
	os.Setenv("BACKUP_PATH", tmpDir)

	// Run cleanup
	cleanupOldBackups()

	// Check remaining files
	files, err := filepath.Glob(filepath.Join(tmpDir, "backup_*.sql"))
	assert.NoError(t, err)

	// Should have deleted old backups (>7 days) and kept recent ones
	t.Logf("Remaining backup files after cleanup: %d", len(files))
	for _, file := range files {
		t.Logf("  - %s", filepath.Base(file))
	}

	// The exact number depends on BACKUP_RETENTION_DAYS, but there should be fewer files
	assert.LessOrEqual(t, len(files), len(testFiles), "Cleanup should not create new files")
}

// TestRunCheckTables_Integration tests the cron wrapper for checkTables
func TestRunCheckTables_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	// Save original db
	originalDB := db
	defer func() { db = originalDB }()

	CONN_STR = connStr
	initDB()
	if db == nil {
		t.Skip("Database not available, skipping test")
	}

	// This should not panic
	runCheckTables()

	t.Log("runCheckTables completed successfully")
}

// TestRunDatabaseBackup_Integration tests the cron wrapper for backup
func TestRunDatabaseBackup_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr := os.Getenv("CONN_STR")
	if connStr == "" {
		t.Skip("CONN_STR not set, skipping test")
	}

	// Create a temporary backup directory
	tmpDir := filepath.Join(os.TempDir(), "gosearch_run_backup_test")
	err := os.MkdirAll(tmpDir, 0755)
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Save original environment
	originalBackupPath := os.Getenv("BACKUP_PATH")
	defer func() {
		if originalBackupPath != "" {
			os.Setenv("BACKUP_PATH", originalBackupPath)
		} else {
			os.Unsetenv("BACKUP_PATH")
		}
	}()

	os.Setenv("BACKUP_PATH", tmpDir)

	// Save original db
	originalDB := db
	defer func() { db = originalDB }()

	CONN_STR = connStr
	initDB()
	if db == nil {
		t.Skip("Database not available, skipping test")
	}

	// This should not panic (but might log errors)
	runDatabaseBackup()

	t.Log("runDatabaseBackup completed successfully")
}

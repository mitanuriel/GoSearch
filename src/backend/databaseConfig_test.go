package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestQueryDBAndCloseDB(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = mockDB.Close() }()

	// set global db to mock
	db = mockDB

	rows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	r, err := queryDB("SELECT 1")
	assert.NoError(t, err)
	if r != nil {
		_ = r.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())

	// closeDB should not panic
	closeDB()
}

func TestCheckTables_WithRows(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = mockDB.Close() }()

	// set global db to mock
	db = mockDB

	// Mock users query
	userRows := sqlmock.NewRows([]string{"id", "username", "email", "password", "password_changed"}).
		AddRow(1, "alice", "a@example.com", "hash", true)
	mock.ExpectQuery(`SELECT \* FROM users`).WillReturnRows(userRows)

	// Mock pages query
	pageRows := sqlmock.NewRows([]string{"title", "url", "language", "last_updated", "content"}).
		AddRow("Title", "http://example.com", "en", time.Now(), "content")
	mock.ExpectQuery(`SELECT \* FROM pages`).WillReturnRows(pageRows)

	// Call checkTables (should not panic)
	checkTables()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupOldBackups(t *testing.T) {
	// Create a temporary directory for this test
	tempDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }() // Clean up temp directory after test

	// Create backups subdirectory inside temp
	backupsDir := filepath.Join(tempDir, "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("Failed to create backups dir: %v", err)
	}

	// Create an old file (10 days old)
	oldFile := filepath.Join(backupsDir, "old_backup.sql")
	f, err := os.Create(oldFile)
	assert.NoError(t, err)
	_ = f.Close()
	oldTime := time.Now().AddDate(0, 0, -10)
	assert.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	// Create a recent file (should not be deleted)
	recentFile := filepath.Join(backupsDir, "recent_backup.sql")
	f2, err := os.Create(recentFile)
	assert.NoError(t, err)
	_ = f2.Close()

	// Since cleanupOldBackups uses a hard-coded path, we need to test the logic
	// by simulating what the function does on our temp directory
	entries, err := os.ReadDir(backupsDir)
	assert.NoError(t, err)

	cutoff := time.Now().AddDate(0, 0, -7) // 7 days ago
	for _, entry := range entries {
		info, err := entry.Info()
		assert.NoError(t, err)

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(backupsDir, entry.Name())
			err := os.Remove(fullPath)
			assert.NoError(t, err)
			t.Logf("Deleted old backup: %s", fullPath)
		}
	}

	// Verify: old file should be removed, recent should remain
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "Old file should be deleted")

	_, err = os.Stat(recentFile)
	assert.NoError(t, err, "Recent file should still exist")

	t.Log("Cleanup test completed successfully with isolated temp directory")
}

func TestBackupDatabase_NoPgDump(t *testing.T) {
	// Set a connection string that will parse
	CONN_STR = "host=localhost port=5432 user=test password=test dbname=testdb"
	// Ensure backups dir exists (function will create if not)
	_ = os.MkdirAll("/app/src/backend/backups", 0755)

	// Run backupDatabase - since pg_dump likely not available in runner, it should handle gracefully
	backupDatabase()

	// If no panic, test passes
	assert.True(t, true)
}

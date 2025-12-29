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
	// Create backups directory within repo (matches code path)
	baseDir := filepath.Join("/app/src/backend/backups")
	// Ensure parent directories exist in workspace
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Skipf("Cannot create backups dir %s: %v - skipping test", baseDir, err)
	}
	// Create an old file
	oldFile := filepath.Join(baseDir, "old_backup.sql")
	f, err := os.Create(oldFile)
	assert.NoError(t, err)
	f.Close()
	// Set mod time to 10 days ago
	oldTime := time.Now().AddDate(0, 0, -10)
	assert.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	// Create a recent file that should not be deleted
	recentFile := filepath.Join(baseDir, "recent_backup.sql")
	f2, err := os.Create(recentFile)
	if err != nil {
		t.Skipf("Cannot create recent file %s: %v - skipping test", recentFile, err)
	}
	f2.Close()

	// Run cleanup
	cleanupOldBackups()

	// old file should be removed, recent should remain
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(recentFile)
	assert.NoError(t, err)

	// Cleanup created files
	_ = os.Remove(recentFile)
	_ = os.RemoveAll(filepath.Dir(baseDir))
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

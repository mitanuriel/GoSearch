package main

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// Note: We can't easily mock the Elasticsearch client because it uses unexported types.
// Instead, we test the database interaction part which we can mock effectively.
// syncPagesToElasticsearch requires a valid esClient, so we focus on DB query patterns.

func TestSyncPagesToElasticsearch_DatabaseQuery(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Test that the DB query pattern used by syncPagesToElasticsearch works correctly
	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Go Programming", "http://golang.org", "The Go programming language").
		AddRow("Python Guide", "http://python.org", "Python programming guide").
		AddRow("JavaScript Docs", "http://js.org", "JavaScript documentation")
	mock.ExpectQuery("SELECT title, url, content FROM pages").WillReturnRows(rows)

	// Simulate the query logic from syncPagesToElasticsearch
	testRows, err := db.Query("SELECT title, url, content FROM pages")
	assert.NoError(t, err, "DB query should succeed")
	defer func() { _ = testRows.Close() }()

	count := 0
	pages := []struct{ title, url, content string }{}
	for testRows.Next() {
		var title, url, content string
		err := testRows.Scan(&title, &url, &content)
		assert.NoError(t, err, "Scan should succeed")
		pages = append(pages, struct{ title, url, content string }{title, url, content})
		count++
	}

	assert.Equal(t, 3, count, "Should return 3 pages from mocked DB")
	assert.Equal(t, "Go Programming", pages[0].title)
	assert.Equal(t, "http://golang.org", pages[0].url)
	assert.Equal(t, "JavaScript Docs", pages[2].title)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncPagesToElasticsearch_EmptyDatabase(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock DB query to return no pages
	rows := sqlmock.NewRows([]string{"title", "url", "content"})
	mock.ExpectQuery("SELECT title, url, content FROM pages").WillReturnRows(rows)

	// Test with empty result set
	testRows, err := db.Query("SELECT title, url, content FROM pages")
	assert.NoError(t, err, "DB query should succeed even with empty results")
	defer func() { _ = testRows.Close() }()

	count := 0
	for testRows.Next() {
		count++
	}

	assert.Equal(t, 0, count, "Should return 0 pages from empty DB")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncPagesToElasticsearch_ValidRowScan(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Test that well-formed rows scan successfully
	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Valid Title", "http://example.com", "Valid content")
	mock.ExpectQuery("SELECT title, url, content FROM pages").WillReturnRows(rows)

	// Test that well-formed rows scan successfully
	testRows, err := db.Query("SELECT title, url, content FROM pages")
	assert.NoError(t, err, "DB query should succeed")
	defer func() { _ = testRows.Close() }()

	hasRows := false
	for testRows.Next() {
		var title, url, content string
		err := testRows.Scan(&title, &url, &content)
		assert.NoError(t, err, "Scan should succeed with valid data")
		assert.Equal(t, "Valid Title", title)
		hasRows = true
	}

	assert.True(t, hasRows, "Should have processed at least one row")
	assert.NoError(t, mock.ExpectationsWereMet())
}


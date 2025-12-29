package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestSearchHandler_NoQuery(t *testing.T) {
	// Test with missing query parameter
	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()

	searchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSearchHandler_EmptyQuery(t *testing.T) {
	// Test with empty query parameter
	req := httptest.NewRequest("GET", "/search?q=", nil)
	w := httptest.NewRecorder()

	searchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSearchHandler_WithValidQuery(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock the search query
	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Test Page", "https://example.com", "Test content about golang")

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%golang%").
		WillReturnRows(rows)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	// Create request
	req := httptest.NewRequest("GET", "/search?q=golang", nil)
	w := httptest.NewRecorder()

	searchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}

func TestSearchHandler_WithWhitespaceQuery(t *testing.T) {
	// Test with query that has leading/trailing whitespace
	req := httptest.NewRequest("GET", "/search?q=++golang++", nil)
	w := httptest.NewRecorder()

	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Test Page", "https://example.com", "Test content")

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%golang%").
		WillReturnRows(rows)

	// Use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	searchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSearchPagesInEs_WithDBFallback(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock the search query
	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Go Programming", "https://golang.org", "Go is a programming language").
		AddRow("Go Tutorial", "https://golang.org/doc", "Learn Go programming")

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%go%").
		WillReturnRows(rows)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	// Execute search
	pages, err := searchPagesInEs("go")

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, pages, 2)
	assert.Equal(t, "Go Programming", pages[0].Title)
	assert.Equal(t, "Go Tutorial", pages[1].Title)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchPagesInEs_NoResults(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock the search query with no results
	rows := sqlmock.NewRows([]string{"title", "url", "content"})

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%nonexistent%").
		WillReturnRows(rows)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	// Execute search
	pages, err := searchPagesInEs("nonexistent")

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, pages, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchPagesInEs_DatabaseError(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock database error
	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%test%").
		WillReturnError(sql.ErrConnDone)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	// Execute search
	pages, err := searchPagesInEs("test")

	// Verify error is returned
	assert.Error(t, err)
	assert.Nil(t, pages)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchPagesInEs_ScanError(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock rows with wrong number of columns (will cause scan error)
	rows := sqlmock.NewRows([]string{"title", "url"}).
		AddRow("Test", "http://test.com") // Missing content column

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%test%").
		WillReturnRows(rows)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	// Execute search
	pages, err := searchPagesInEs("test")

	// Should not error but skip bad rows
	assert.NoError(t, err)
	assert.Len(t, pages, 0) // Bad row skipped
}

func TestSearchHandler_TemplateError(t *testing.T) {
	// Temporarily modify templatePath to cause template error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	// Setup mock DB for searchPagesInEs
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock DB query to return results
	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Test Page", "http://test.com", "Test content")
	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%test%").
		WillReturnRows(rows)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	req := httptest.NewRequest("GET", "/search?q=test", nil)
	w := httptest.NewRecorder()

	searchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestSearchHandler_SearchError(t *testing.T) {
	// Setup mock DB to return error
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%test%").
		WillReturnError(assert.AnError)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	req := httptest.NewRequest("GET", "/search?q=test", nil)
	w := httptest.NewRecorder()

	searchHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestSearchPagesInEs_MultipleResults(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock multiple rows
	rows := sqlmock.NewRows([]string{"title", "url", "content"}).
		AddRow("Page 1", "http://test1.com", "Content 1").
		AddRow("Page 2", "http://test2.com", "Content 2").
		AddRow("Page 3", "http://test3.com", "Content 3")

	mock.ExpectQuery("SELECT title, url, content FROM pages WHERE content LIKE ?").
		WithArgs("%golang%").
		WillReturnRows(rows)

	// Set esClient to nil to use DB fallback
	originalEsClient := esClient
	esClient = nil
	defer func() { esClient = originalEsClient }()

	// Execute search
	pages, err := searchPagesInEs("golang")

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, pages, 3)
	assert.Equal(t, "Page 1", pages[0].Title)
	assert.Equal(t, "Page 2", pages[1].Title)
	assert.Equal(t, "Page 3", pages[2].Title)
}

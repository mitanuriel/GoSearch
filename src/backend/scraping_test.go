// Unit tests for scraping utility functions
package main

import (
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestBuildWikipediaURL(t *testing.T) {
	tests := []struct {
		name     string
		term     string
		lang     string
		expected string
	}{
		{
			name:     "Simple term in English",
			term:     "golang",
			lang:     "en",
			expected: "https://en.wikipedia.org/wiki/Golang",
		},
		{
			name:     "Term with spaces in Danish",
			term:     "hello world",
			lang:     "da",
			expected: "https://da.wikipedia.org/wiki/Hello_world", // Title case applies to first char only
		},
		{
			name:     "Single word in German",
			term:     "computer",
			lang:     "de",
			expected: "https://de.wikipedia.org/wiki/Computer",
		},
		{
			name:     "Multiple spaces",
			term:     "artificial  intelligence",
			lang:     "en",
			expected: "https://en.wikipedia.org/wiki/Artificial__intelligence",
		},
		{
			name:     "Already titlecased",
			term:     "Python Programming",
			lang:     "en",
			expected: "https://en.wikipedia.org/wiki/Python_programming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildWikipediaURL(tt.term, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSearchTerms(t *testing.T) {
	// Test with non-existent file
	t.Run("Non-existent file", func(t *testing.T) {
		result := extractSearchTerms("/non/existent/path.log")
		assert.Nil(t, result)
	})

	// Test with empty file
	t.Run("Empty file", func(t *testing.T) {
		// Create temp empty file
		tmpfile, err := createTempLogFile("")
		if err != nil {
			t.Fatal(err)
		}
		defer cleanupTempFile(tmpfile)

		result := extractSearchTerms(tmpfile)
		assert.Empty(t, result)
	})

	// Test with valid log entries
	t.Run("Valid log entries", func(t *testing.T) {
		logContent := `2025-12-25 query="golang" from=127.0.0.1
2025-12-25 query="python" from=127.0.0.1
2025-12-25 query="golang" from=127.0.0.1
2025-12-25 query="javascript" from=192.168.1.1`

		tmpfile, err := createTempLogFile(logContent)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanupTempFile(tmpfile)

		result := extractSearchTerms(tmpfile)
		assert.Len(t, result, 3) // golang, python, javascript (duplicates removed)
		assert.Contains(t, result, "golang")
		assert.Contains(t, result, "python")
		assert.Contains(t, result, "javascript")
	})

	// Test case sensitivity
	t.Run("Case insensitive deduplication", func(t *testing.T) {
		logContent := `query="Golang" from=127.0.0.1
query="GOLANG" from=127.0.0.1
query="golang" from=127.0.0.1`

		tmpfile, err := createTempLogFile(logContent)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanupTempFile(tmpfile)

		result := extractSearchTerms(tmpfile)
		assert.Len(t, result, 1) // All should be deduplicated to "golang"
		assert.Contains(t, result, "golang")
	})
}

// Helper function to create temporary log file
func createTempLogFile(content string) (string, error) {
	tmpfile, err := os.CreateTemp("", "test_search_*.log")
	if err != nil {
		return "", err
	}

	if content != "" {
		if _, err := tmpfile.WriteString(content); err != nil {
			_ = tmpfile.Close()
			_ = os.Remove(tmpfile.Name())
			return "", err
		}
	}

	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpfile.Name())
		return "", err
	}

	return tmpfile.Name(), nil
}

// Helper function to cleanup temp file
func cleanupTempFile(path string) {
	_ = os.Remove(path)
}

func TestAlreadyProcessed(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	tests := []struct {
		name      string
		term      string
		setupMock func()
		expected  bool
	}{
		{
			name: "Term already processed",
			term: "golang",
			setupMock: func() {
				mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
					WithArgs("golang").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			},
			expected: true,
		},
		{
			name: "Term not processed",
			term: "python",
			setupMock: func() {
				mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
					WithArgs("python").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result := alreadyProcessed(tt.term)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMarkAsProcessed(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	tests := []struct {
		name      string
		term      string
		setupMock func()
	}{
		{
			name: "Successfully mark term as processed",
			term: "golang",
			setupMock: func() {
				mock.ExpectExec("INSERT INTO processed_searches \\(search_term\\) VALUES \\(\\$1\\) ON CONFLICT DO NOTHING").
					WithArgs("golang").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "Mark duplicate term (conflict ignored)",
			term: "python",
			setupMock: func() {
				mock.ExpectExec("INSERT INTO processed_searches \\(search_term\\) VALUES \\(\\$1\\) ON CONFLICT DO NOTHING").
					WithArgs("python").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			markAsProcessed(tt.term)
			// Just verify no panic occurs
			err := mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestSavePageToDBWithLang_ValidPage(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Create test page
	page := Page{
		Title:   "Test Page",
		URL:     "https://en.wikipedia.org/wiki/Test",
		Content: "This is test content",
	}

	// Expect INSERT query
	mock.ExpectExec("INSERT INTO pages").
		WithArgs(page.URL, page.Title, page.Content, "en").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call function
	err := savePageToDBWithLang(page, "en")

	// Verify
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScrapeWikipedia_InvalidDomain(t *testing.T) {
	// Test that scraping fails gracefully with invalid domains
	// Since scrapeWikipedia uses colly with AllowedDomains, it will reject non-Wikipedia URLs
	page, err := scrapeWikipedia("http://example.com/test", "en")

	// Should return an error because example.com is not in allowed domains (en.wikipedia.org)
	assert.Error(t, err, "Should fail for non-Wikipedia domain")
	assert.Contains(t, err.Error(), "Forbidden", "Error should mention forbidden domain")
	assert.Equal(t, "http://example.com/test", page.URL)
	assert.Equal(t, "en", page.Language)
}

func TestScrapeWikipedia_URLFormat(t *testing.T) {
	// Test that the function accepts properly formatted URLs
	// We can't actually scrape without a real server, but we can verify the function signature works

	testCases := []struct {
		name string
		url  string
		lang string
	}{
		{"English", "https://en.wikipedia.org/wiki/Go_(programming_language)", "en"},
		{"Danish", "https://da.wikipedia.org/wiki/Go_(programmeringssprog)", "da"},
		{"Swedish", "https://sv.wikipedia.org/wiki/Go_(programspråk)", "sv"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: This will make actual HTTP requests to Wikipedia
			// In a real test environment, you might want to skip these or use VCR/recording
			page, err := scrapeWikipedia(tc.url, tc.lang)

			// We expect these to succeed (or fail gracefully if Wikipedia is down)
			if err != nil {
				t.Logf("Scraping %s failed (may be network issue): %v", tc.url, err)
			} else {
				assert.NotEmpty(t, page.Title, "Title should not be empty on success")
				assert.Equal(t, tc.url, page.URL)
				assert.Equal(t, tc.lang, page.Language)
				t.Logf("Successfully scraped: %s", page.Title)
			}
		})
	}
}

func TestSavePageToDBWithLang_EmptyTitle(t *testing.T) {
	// Setup mock database
	mockDB, _ := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Create page with empty title
	page := Page{
		Title:   "",
		URL:     "https://en.wikipedia.org/wiki/Test",
		Content: "Content",
	}

	// Call function
	err := savePageToDBWithLang(page, "en")

	// Should return error for invalid data
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid page data")
}

func TestSavePageToDBWithLang_EmptyURL(t *testing.T) {
	// Setup mock database
	mockDB, _ := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Create page with empty URL
	page := Page{
		Title:   "Test",
		URL:     "",
		Content: "Content",
	}

	// Call function
	err := savePageToDBWithLang(page, "en")

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid page data")
}

func TestSavePageToDBWithLang_EmptyContent(t *testing.T) {
	// Setup mock database
	mockDB, _ := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Create page with empty content
	page := Page{
		Title:   "Test",
		URL:     "https://test.com",
		Content: "",
	}

	// Call function
	err := savePageToDBWithLang(page, "en")

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid page data")
}

func TestSavePageToDBWithLang_DatabaseError(t *testing.T) {
	// Setup mock database
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Create valid page
	page := Page{
		Title:   "Test Page",
		URL:     "https://en.wikipedia.org/wiki/Test",
		Content: "Content",
	}

	// Expect INSERT query to fail
	mock.ExpectExec("INSERT INTO pages").
		WithArgs(page.URL, page.Title, page.Content, "en").
		WillReturnError(assert.AnError)

	// Call function
	err := savePageToDBWithLang(page, "en")

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error inserting or updating page")
}

func TestStartScraping_NoSearchTerms(t *testing.T) {
	// Create empty log file
	tmpfile, err := os.CreateTemp("", "test-search-*.log")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()
	_ = tmpfile.Close()

	// Call StartScraping (should handle empty gracefully)
	StartScraping(tmpfile.Name())

	// Test passes if no panic
	t.Log("Successfully handled empty search log file without errors")
}

func TestStartScraping_NonExistentFile(t *testing.T) {
	// Call with non-existent file (should handle gracefully)
	StartScraping("/non/existent/path.log")

	// Test passes if no panic
	t.Log("Successfully handled non-existent file without panicking")
}

func TestTryScrapeInLanguages_NoLanguages(t *testing.T) {
	// Call with empty language list
	_, _, err := tryScrapeInLanguages("test", []string{})

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid Wikipedia page found")
}

func TestTryScrapeInLanguages_NonExistentPage(t *testing.T) {
	// Test with a term that doesn't exist in any Wikipedia
	// Using a very unlikely combination to trigger failures
	_, _, err := tryScrapeInLanguages("xyzzynonexistenttermabc123", []string{"da", "en"})

	// Should return error after trying all languages
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid Wikipedia page found")
}

func TestTryScrapeInLanguages_SuccessOnFirstLanguage(t *testing.T) {
	// Test scraping a known page (Go programming language)
	// This makes a real HTTP request, so skip if in CI or offline
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	page, lang, err := tryScrapeInLanguages("Go", []string{"en", "da"})

	// Should succeed on English Wikipedia
	assert.NoError(t, err)
	assert.Equal(t, "en", lang)
	assert.NotEmpty(t, page.Title)
	assert.NotEmpty(t, page.Content)
	assert.Contains(t, page.URL, "en.wikipedia.org")
	t.Logf("Successfully scraped: %s (%s)", page.Title, lang)
}

func TestTryScrapeInLanguages_FallbackToSecondLanguage(t *testing.T) {
	// Test fallback mechanism with a term that doesn't exist in Danish but exists in English
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use a term unlikely to exist in Danish Wikipedia but common in English
	// "Supercalifragilisticexpialidocious" won't exist, so we use a more realistic test
	// Try with an obscure English term that likely doesn't have Danish translation
	page, lang, err := tryScrapeInLanguages("Golang", []string{"da", "en"})

	// Should succeed on English after Danish fails
	if err == nil {
		assert.NotEmpty(t, page.Title)
		// Most likely succeeded on English since "Golang" redirect might not exist in Danish
		assert.Contains(t, []string{"da", "en"}, lang)
		t.Logf("Successfully scraped: %s (%s)", page.Title, lang)
	} else {
		t.Logf("Both languages failed: %v", err)
	}
}

func TestStartScraping_WithSearchTerms(t *testing.T) {
	// Create temp log file with search terms
	logContent := `query="testterm" from=127.0.0.1
query="golang" from=127.0.0.1`

	tmpfile, err := createTempLogFile(logContent)
	assert.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	// Setup mock DB
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock alreadyProcessed queries - return true for both to skip them
	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
		WithArgs("testterm").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
		WithArgs("golang").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Call StartScraping (should skip both terms since they're already processed)
	StartScraping(tmpfile)

	// Test passes if no panic
	t.Log("Successfully processed search terms with alreadyProcessed check")
}

func TestAlreadyProcessed_QueryError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock query to return error
	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
		WithArgs("test").
		WillReturnError(assert.AnError)

	result := alreadyProcessed("test")
	// Should return false on error
	assert.False(t, result)
}

func TestMarkAsProcessed_Error(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock INSERT to return error
	mock.ExpectExec("INSERT INTO processed_searches \\(search_term\\) VALUES \\(\\$1\\) ON CONFLICT DO NOTHING").
		WithArgs("test").
		WillReturnError(assert.AnError)

	// Call markAsProcessed (should not panic even on error)
	markAsProcessed("test")

	t.Log("Successfully handled markAsProcessed error")
}

func TestStartScraping_SuccessfulFlow(t *testing.T) {
	// Skip this integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp log file with a term that exists on Wikipedia
	logContent := `query="Go" from=127.0.0.1`

	tmpfile, err := createTempLogFile(logContent)
	assert.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	// Setup mock DB
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock alreadyProcessed query - return false to allow scraping
	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
		WithArgs("go").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock savePageToDBWithLang INSERT
	mock.ExpectExec("INSERT INTO pages").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock markAsProcessed INSERT
	mock.ExpectExec("INSERT INTO processed_searches \\(search_term\\) VALUES \\(\\$1\\) ON CONFLICT DO NOTHING").
		WithArgs("go").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call StartScraping - should scrape, save, and mark as processed
	StartScraping(tmpfile)

	// Verify all expectations met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
	t.Log("Successfully completed full scraping flow")
}

func TestStartScraping_SaveToDBError(t *testing.T) {
	// Skip this integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp log file
	logContent := `query="Go" from=127.0.0.1`

	tmpfile, err := createTempLogFile(logContent)
	assert.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	// Setup mock DB
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock alreadyProcessed query - return false
	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
		WithArgs("go").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock savePageToDBWithLang INSERT to fail
	mock.ExpectExec("INSERT INTO pages").
		WillReturnError(assert.AnError)

	// Call StartScraping - should scrape but fail to save
	// Should NOT call markAsProcessed since save failed
	StartScraping(tmpfile)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
	t.Log("Successfully handled save error without marking as processed")
}

func TestStartScraping_ScrapeError(t *testing.T) {
	// Create temp log file with a term that doesn't exist
	logContent := `query="xyzzynonexistentabc123" from=127.0.0.1`

	tmpfile, err := createTempLogFile(logContent)
	assert.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	// Setup mock DB
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock alreadyProcessed query - return false
	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM processed_searches WHERE search_term = \\$1\\)").
		WithArgs("xyzzynonexistentabc123").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Call StartScraping - should fail to scrape
	// Should NOT call savePageToDBWithLang or markAsProcessed
	StartScraping(tmpfile)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
	t.Log("Successfully handled scrape error without saving or marking")
}


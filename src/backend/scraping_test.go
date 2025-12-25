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

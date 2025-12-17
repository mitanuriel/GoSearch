//go:build integration
// +build integration

// Integration tests that require external dependencies (Elasticsearch, DB)
// Run with: go test -tags=integration ./src/backend/...
package main

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	// Import SQLite driver for database/sql - registers driver via init()
	_ "github.com/mattn/go-sqlite3"

	"github.com/gorilla/sessions"
)

// setupTestDB initializes an in-memory SQLite DB and schema
func setupTestDB(t *testing.T) {
	var err error
	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	schema := `
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT,
    email TEXT,
    password TEXT,
	password_changed BOOLEAN DEFAULT TRUE
);
CREATE TABLE pages (
    title TEXT,
    url TEXT,
    language TEXT,
    last_updated DATETIME,
    content TEXT
);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
}

// runTest is a table-driven helper for integration scenarios.
func runTest(t *testing.T, name, method, path string, form url.Values, seed func(), check func(*http.Response, string)) {
	t.Run(name, func(t *testing.T) {
		setupTestDB(t)
		if seed != nil {
			seed()
		}
		// init session store
		store = sessions.NewCookieStore([]byte("test-secret"))

		ts := httptest.NewServer(setupRouter())
		defer ts.Close()

		client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}

		// perform request
		var resp *http.Response
		var err error
		url := ts.URL + path
		switch method {
		case http.MethodGet:
			resp, err = client.Get(url)
		case http.MethodPost:
			resp, err = client.PostForm(url, form)
		default:
			t.Fatalf("unsupported method %s", method)
		}
		if err != nil {
			t.Fatalf("%s request error: %v", name, err)
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)
		check(resp, body)
	})
}

func TestIntegration(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		form   url.Values
		seed   func()
		check  func(*http.Response, string)
	}{
		{
			name:   "Weather",
			method: http.MethodGet,
			path:   "/api/weather?city=Copenhagen",
			check: func(resp *http.Response, body string) {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Weather expected 200 OK, got %d", resp.StatusCode)
				}
				if !strings.Contains(body, "Copenhagen") {
					t.Errorf("Weather expected city in body, got %s", body)
				}
			},
		},
		{
			name:   "Root",
			method: http.MethodGet,
			path:   "/",
			check: func(resp *http.Response, body string) {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Root expected 200 OK, got %d", resp.StatusCode)
				}
				// index.html displays site title
				expectedTitle := "Home - ¿Go Search?"
				if !strings.Contains(body, expectedTitle) {
					t.Errorf("Root expected site title '%s' in body, got %s", expectedTitle, body)
				}
			},
		},
		{
			name:   "Search",
			method: http.MethodGet,
			path:   "/api/search?q=TestContent&language=en",
			seed: func() {
				// Skip this test if Elasticsearch is not available
				if esHost == "" {
					t.Skip("Skipping Search test: Elasticsearch not available (ES_HOST not set)")
				}
				if _, err := db.Exec(
					`INSERT INTO pages (title,url,language,last_updated,content) VALUES (?,?,?,?,?);`,
					"TestTitle", "/test-url", "en", "2025-01-01", "TestContent",
				); err != nil {
					t.Fatalf("Search seed failed: %v", err)
				}
				if err := syncPagesToElasticsearch(); err != nil {
					t.Fatalf("Failed to sync pages to Elasticsearch: %v", err)
				}
			},
			check: func(resp *http.Response, body string) {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Search expected 200 OK, got %d", resp.StatusCode)
				}
				if !strings.Contains(body, "TestTitle") {
					t.Errorf("Search expected 'TestTitle', got %s", body)
				}
			},
		},
		{
			name:   "API Login",
			method: http.MethodPost,
			path:   "/api/login",
			form:   url.Values{"username": {"user1"}, "password": {"pass123"}},
			seed: func() {
				hash, _ := hashPassword("pass123")
				if _, err := db.Exec(
					`INSERT INTO users (username,email,password,password_changed) VALUES (?,?,?,?);`,
					"user1", "u1@example.com", hash, true,
				); err != nil {
					t.Fatalf("Login seed failed: %v", err)
				}
			},
			check: func(resp *http.Response, body string) {
				if resp.StatusCode != http.StatusSeeOther {
					t.Errorf("API Login expected 303 SeeOther, got %d", resp.StatusCode)
				}
			},
		},
		{
			name:   "API Register",
			method: http.MethodPost,
			path:   "/api/register",
			form:   url.Values{"username": {"newuser"}, "email": {"new@example.com"}, "password": {"abc123"}, "password2": {"abc123"}},
			check: func(resp *http.Response, body string) {
				if resp.StatusCode != http.StatusSeeOther {
					t.Errorf("API Register expected 303 SeeOther, got %d", resp.StatusCode)
				}
			},
		},
	}

	for _, tc := range cases {
		runTest(t, tc.name, tc.method, tc.path, tc.form, tc.seed, tc.check)
	}
}

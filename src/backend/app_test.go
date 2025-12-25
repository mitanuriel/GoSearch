// Unit tests for authentication handlers
// These tests use mocks and don't require external dependencies
// Run with: go test ./src/backend/... (default - no build tags needed)
package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// Helper function to create a mock database
func setupMockDB() (*sql.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		panic(err)
	}

	db = mockDB
	return mockDB, mock
}

func TestLoginSuccess(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	testUsername := "testUser"
	testPassword := "testPassword"

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT id, username, password FROM users WHERE username = ?").
		WithArgs(testUsername).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).
		AddRow(1, testUsername, string(hashedPassword)))

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	formData := url.Values{
		"username": {testUsername},
		"password": {testPassword},
	}

	req, err := http.NewRequest("POST", "/api/login", strings.NewReader(formData.Encode()))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	apiLogin(w, req)

	response := w.Result()

	assert.Equal(t, http.StatusSeeOther, response.StatusCode, "Expected redirect after successful login")

}

// Failed login
func TestFailedLogin(t *testing.T) {
	testCases := []struct {
		name           string
		username       string
		password       string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectedStatus int
	}{
		{
			name:           "Empty Username",
			username:       "",
			password:       "testPassword",
			mockSetup:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Empty password",
			username:       "testUser",
			password:       "",
			mockSetup:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:     "Incorrect password",
			username: "testUser",
			password: "invalidPassword",
			mockSetup: func(mock sqlmock.Sqlmock) {
				hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("validPassword"), bcrypt.DefaultCost)
				mock.ExpectQuery("SELECT id, username, password FROM users WHERE username = ?").WithArgs("testUser").
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).AddRow(1, "testUser", string(hashedPassword)))
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:     "User not found",
			username: "NonExistentUser",
			password: "randomPassword",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, username, password FROM users WHERE username = ?").WithArgs("NonExistentUser").
					WillReturnError(sql.ErrNoRows)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	//Setup mock database for each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB, mock := setupMockDB()
			defer func() { _ = mockDB.Close() }()

			//Setup mock based on test case
			tc.mockSetup(mock)

			//Create a mock session store
			mockStore := sessions.NewCookieStore([]byte("test-key"))
			store = mockStore

			//Create form data
			formdata := url.Values{
				"username": {tc.username},
				"password": {tc.password},
			}

			//Create a request
			req, err := http.NewRequest("POST", "/api/login", strings.NewReader(formdata.Encode()))
			assert.NoError(t, err)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			//Create a response recorder
			w := httptest.NewRecorder()

			// Run apilogin function
			apiLogin(w, req)

			//Check response
			response := w.Result()

			assert.Equal(t, tc.expectedStatus, response.StatusCode, "Unexpected response status for "+tc.name)

		})
	}

}

// Logout test
func TestLogout(t *testing.T) {

	// Create a mock session store
	mockStore := sessions.NewCookieStore([]byte("secret-test-key"))

	store = mockStore

	// Create a mock request
	req, err := http.NewRequest("GET", "/api/logout", nil)
	assert.NoError(t, err)

	// Create response recorder
	w := httptest.NewRecorder()

	// Create session with logged in user
	session, err := store.Get(req, "session-name")
	assert.NoError(t, err)

	// Add user id
	session.Values["user_id"] = 1

	// Save session
	err = session.Save(req, w)
	assert.NoError(t, err)

	// Call logout function
	logoutHandler(w, req)

	// Get the result
	response := w.Result()

	// Check the status code
	assert.Equal(t, http.StatusSeeOther, response.StatusCode, "Redirect is expected")

	// Get the updated session
	updatedSession, err := store.Get(req, "session-name")
	assert.NoError(t, err)

	// Check that the user id is gone
	_, ok := updatedSession.Values["user_id"]
	assert.False(t, ok, "User id should no longer exist in a session")

	// Check that session is set to expire
	assert.Equal(t, -1, updatedSession.Options.MaxAge, "Session should be set to expire")
}

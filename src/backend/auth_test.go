// Unit tests for authentication utility functions
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "testPassword123"
	
	hashed, err := hashPassword(password)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, hashed)
	assert.NotEqual(t, password, hashed, "Hashed password should not match plain password")
	
	// Verify the hashed password can be validated
	err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	assert.NoError(t, err, "Hashed password should validate against original")
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	// Test with empty password
	hashed, err := hashPassword("")
	assert.NoError(t, err, "bcrypt allows empty passwords")
	assert.NotEmpty(t, hashed)
}

func TestHashPassword_LongPassword(t *testing.T) {
	// Test with very long password (bcrypt has 72 byte limit)
	longPassword := ""
	for i := 0; i < 100; i++ {
		longPassword += "a"
	}
	hashed, err := hashPassword(longPassword)
	// bcrypt will return an error for passwords > 72 bytes
	assert.Error(t, err, "bcrypt should error on passwords exceeding 72 bytes")
	assert.Empty(t, hashed)
	assert.Contains(t, err.Error(), "password length exceeds 72 bytes")
}

func TestValidatePassword(t *testing.T) {
	password := "testPassword123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	assert.NoError(t, err)

	tests := []struct {
		name           string
		hashedPassword string
		inputPassword  string
		expected       bool
	}{
		{
			name:           "Correct password",
			hashedPassword: string(hashedPassword),
			inputPassword:  password,
			expected:       true,
		},
		{
			name:           "Incorrect password",
			hashedPassword: string(hashedPassword),
			inputPassword:  "wrongPassword",
			expected:       false,
		},
		{
			name:           "Empty password",
			hashedPassword: string(hashedPassword),
			inputPassword:  "",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePassword(tt.hashedPassword, tt.inputPassword)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUserExists(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	tests := []struct {
		name              string
		username          string
		email             string
		setupMock         func()
		expectedUsername  bool
		expectedEmail     bool
	}{
		{
			name:     "Both username and email exist",
			username: "existingUser",
			email:    "existing@example.com",
			setupMock: func() {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
					WithArgs("existingUser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
					WithArgs("existing@example.com").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				mock.ExpectCommit()
			},
			expectedUsername: true,
			expectedEmail:    true,
		},
		{
			name:     "Username exists, email does not",
			username: "existingUser",
			email:    "new@example.com",
			setupMock: func() {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
					WithArgs("existingUser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
					WithArgs("new@example.com").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				mock.ExpectCommit()
			},
			expectedUsername: true,
			expectedEmail:    false,
		},
		{
			name:     "Neither exists",
			username: "newUser",
			email:    "new@example.com",
			setupMock: func() {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
					WithArgs("newUser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
					WithArgs("new@example.com").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				mock.ExpectCommit()
			},
			expectedUsername: false,
			expectedEmail:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			usernameExists, emailExists := userExists(tt.username, tt.email)
			assert.Equal(t, tt.expectedUsername, usernameExists)
			assert.Equal(t, tt.expectedEmail, emailExists)
		})
	}
}

func TestUserExists_BeginError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Test when db.Begin() fails
	mock.ExpectBegin().WillReturnError(assert.AnError)

	usernameExists, emailExists := userExists("test", "test@example.com")
	assert.False(t, usernameExists)
	assert.False(t, emailExists)
}

func TestUserExists_UsernameQueryError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Test when username query fails
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("test").
		WillReturnError(assert.AnError)
	mock.ExpectRollback()

	usernameExists, emailExists := userExists("test", "test@example.com")
	assert.False(t, usernameExists)
	assert.False(t, emailExists)
}

func TestUserExists_EmailQueryError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Test when email query fails
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("test").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("test@example.com").
		WillReturnError(assert.AnError)
	mock.ExpectRollback()

	usernameExists, emailExists := userExists("test", "test@example.com")
	assert.False(t, usernameExists)
	assert.False(t, emailExists)
}

func TestUserExists_CommitError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Test when commit fails
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("test").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectCommit().WillReturnError(assert.AnError)

	usernameExists, emailExists := userExists("test", "test@example.com")
	assert.False(t, usernameExists)
	assert.False(t, emailExists)
}

func TestUserIsLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	tests := []struct {
		name     string
		setupReq func() *http.Request
		expected bool
	}{
		{
			name: "User is logged in",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				w := httptest.NewRecorder()
				session, _ := store.Get(req, "session-name")
				session.Values["user_id"] = 1
				_ = session.Save(req, w)
				// Copy cookies from response to request
				for _, cookie := range w.Result().Cookies() {
					req.AddCookie(cookie)
				}
				return req
			},
			expected: true,
		},
		{
			name: "User is not logged in",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			expected: false,
		},
		{
			name: "Session without user_id",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				w := httptest.NewRecorder()
				session, _ := store.Get(req, "session-name")
				session.Values["other_key"] = "value"
				_ = session.Save(req, w)
				// Copy cookies from response to request
				for _, cookie := range w.Result().Cookies() {
					req.AddCookie(cookie)
				}
				return req
			},
			expected: false,
		},
		{
			name: "Session with nil user_id",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				w := httptest.NewRecorder()
				session, _ := store.Get(req, "session-name")
				session.Values["user_id"] = nil
				_ = session.Save(req, w)
				// Copy cookies from response to request
				for _, cookie := range w.Result().Cookies() {
					req.AddCookie(cookie)
				}
				return req
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			result := userIsLoggedIn(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "Valid .com email",
			email:    "test@example.com",
			expected: true,
		},
		{
			name:     "Valid .dk email",
			email:    "user@domain.dk",
			expected: true,
		},
		{
			name:     "Valid .org email",
			email:    "contact@organization.org",
			expected: true,
		},
		{
			name:     "Valid .net email",
			email:    "admin@network.net",
			expected: true,
		},
		{
			name:     "Valid .edu email",
			email:    "student@university.edu",
			expected: true,
		},
		{
			name:     "Email with spaces (should trim)",
			email:    "  test@example.com  ",
			expected: true,
		},
		{
			name:     "Invalid - no @ symbol",
			email:    "testexample.com",
			expected: false,
		},
		{
			name:     "Invalid - starts with @",
			email:    "@example.com",
			expected: false,
		},
		{
			name:     "Invalid - ends with @",
			email:    "test@",
			expected: false,
		},
		{
			name:     "Invalid - multiple @ symbols",
			email:    "test@@example.com",
			expected: false,
		},
		{
			name:     "Invalid - unsupported TLD",
			email:    "test@example.xyz",
			expected: false,
		},
		{
			name:     "Invalid - empty string",
			email:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEmail(tt.email)
			assert.Equal(t, tt.expected, result, "Email: %s", tt.email)
		})
	}
}

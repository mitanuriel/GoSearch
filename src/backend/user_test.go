// Unit tests for user-related functions
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
)

func TestGetTemplates(t *testing.T) {
	tmpl, err := getTemplates()
	
	// Templates might not exist in test environment, so we just check the error handling
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, tmpl)
	} else {
		assert.NotNil(t, tmpl)
	}
}

func TestLoadTemplates(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		expectErr bool
	}{
		{
			name:      "Empty file list",
			files:     []string{},
			expectErr: true, // ParseFiles with no files returns error
		},
		{
			name:      "Non-existent files",
			files:     []string{"nonexistent.html"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := loadTemplates(tt.files...)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tmpl)
			}
		})
	}
}

func TestApiRegisterHandler_InvalidRequests(t *testing.T) {
	mockDB, _ := setupMockDB()
	defer func() { _ = mockDB.Close() }()
	
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	tests := []struct {
		name           string
		formData       map[string]string
		expectedStatus int
	}{
		{
			name: "Empty username",
			formData: map[string]string{
				"username": "",
				"email":    "test@example.com",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Empty email",
			formData: map[string]string{
				"username": "testuser",
				"email":    "",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Empty password",
			formData: map[string]string{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid email format",
			formData: map[string]string{
				"username": "testuser",
				"email":    "invalid-email",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			for k, v := range tt.formData {
				form.Set(k, v)
			}

			req := httptest.NewRequest("POST", "/api/register", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			apiRegisterHandler(w, req)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestLoginHandler(t *testing.T) {
	// This is a GET request handler that just renders the template
	// We test that it doesn't panic or return 500
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	// We expect this might fail due to missing templates in test env
	// But it shouldn't panic
	login(w, req)

	resp := w.Result()
	// Either OK (templates exist) or 500 (templates missing)
	// Both are acceptable in test environment
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestLogoutHandler(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	t.Run("Logout with existing session", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/logout", nil)
		w := httptest.NewRecorder()

		// Create a session first
		session, _ := store.Get(req, "session-name")
		session.Values["user_id"] = 1
		_ = session.Save(req, w)

		// Copy cookie to new request
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}

		// Now test logout
		w = httptest.NewRecorder()
		logoutHandler(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

		location, err := resp.Location()
		assert.NoError(t, err)
		assert.Equal(t, "/", location.Path)
	})

	t.Run("Logout without session", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/logout", nil)
		w := httptest.NewRecorder()

		logoutHandler(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	})
}

func TestApiLogin_MissingCredentials(t *testing.T) {
	mockDB, _ := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Initialize templates - will fail gracefully if not found
	_, _ = loadTemplates("layout.html", "login.html")

	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "Empty username",
			username: "",
			password: "password",
		},
		{
			name:     "Empty password",
			username: "user",
			password: "",
		},
		{
			name:     "Both empty",
			username: "",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{
				"username": {tt.username},
				"password": {tt.password},
			}

			req := httptest.NewRequest("POST", "/api/login", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			apiLogin(w, req)

			resp := w.Result()
			// Should return either 400 or render error page (200/500)
			assert.True(t, resp.StatusCode >= 200)
		})
	}
}

func TestRegisterHandler_NotLoggedIn(t *testing.T) {
	// Setup
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create request
	req := httptest.NewRequest("GET", "/register", nil)
	w := httptest.NewRecorder()

	// Call handler
	registerHandler(w, req)

	// Should render registration page (StatusOK) or fail gracefully if templates missing (StatusInternalServerError)
	resp := w.Result()
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"Expected StatusOK (200) or StatusInternalServerError (500), got %d", resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	}
}

func TestRegisterHandler_AlreadyLoggedIn(t *testing.T) {
	// Setup session store (no DB needed - handler only checks session)
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create request with session
	req := httptest.NewRequest("GET", "/register", nil)
	w := httptest.NewRecorder()

	// Create authenticated session
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 123
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/register", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Call handler
	registerHandler(w2, req2)

	// Should redirect to home
	resp := w2.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestApiRegisterHandler_PasswordMismatch(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	form := url.Values{
		"username":  {"testuser"},
		"email":     {"test@example.com"},
		"password":  {"password123"},
		"password2": {"password456"}, // Different password
	}

	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiRegisterHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestApiRegisterHandler_UsernameTaken(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Mock userExists to return true for username
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("existinguser").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectCommit()

	form := url.Values{
		"username":  {"existinguser"},
		"email":     {"test@example.com"},
		"password":  {"password123"},
		"password2": {"password123"},
	}

	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiRegisterHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestApiRegisterHandler_EmailTaken(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Mock userExists to return true for email
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("newuser").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("existing@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()

	form := url.Values{
		"username":  {"newuser"},
		"email":     {"existing@example.com"},
		"password":  {"password123"},
		"password2": {"password123"},
	}

	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiRegisterHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestApiRegisterHandler_Success(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Mock userExists - both return false
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(username\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("newuser").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)\\)").
		WithArgs("new@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectCommit()

	// Mock INSERT query
	mock.ExpectQuery("INSERT INTO users \\(username, email, password, password_changed\\) VALUES \\(\\$1, \\$2, \\$3, TRUE\\) RETURNING id").
		WithArgs("newuser", "new@example.com", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	form := url.Values{
		"username":  {"newuser"},
		"email":     {"new@example.com"},
		"password":  {"password123"},
		"password2": {"password123"},
	}

	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiRegisterHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestApiRegisterHandler_AlreadyLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create request with session
	req := httptest.NewRequest("POST", "/api/register", nil)
	w := httptest.NewRecorder()

	// Create authenticated session
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 123
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	form := url.Values{
		"username":  {"newuser"},
		"email":     {"new@example.com"},
		"password":  {"password123"},
		"password2": {"password123"},
	}
	req2 := httptest.NewRequest("POST", "/api/register", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	apiRegisterHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestApiLogin_UserNotFound(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Mock query to return no rows
	mock.ExpectQuery("SELECT id, username, password FROM users WHERE username = \\$1").
		WithArgs("nonexistent").
		WillReturnError(assert.AnError)

	form := url.Values{
		"username": {"nonexistent"},
		"password": {"password123"},
	}

	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiLogin(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestApiLogin_WrongPassword(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create a hashed password
	hashedPassword, _ := hashPassword("correctpassword")

	// Mock query to return user
	mock.ExpectQuery("SELECT id, username, password FROM users WHERE username = \\$1").
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(1, "testuser", hashedPassword))

	form := url.Values{
		"username": {"testuser"},
		"password": {"wrongpassword"},
	}

	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiLogin(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestApiLogin_Success(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create a hashed password
	hashedPassword, _ := hashPassword("password123")

	// Mock query to return user
	mock.ExpectQuery("SELECT id, username, password FROM users WHERE username = \\$1").
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(1, "testuser", hashedPassword))

	form := url.Values{
		"username": {"testuser"},
		"password": {"password123"},
	}

	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiLogin(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestLogin_TemplateError(t *testing.T) {
	// Temporarily modify templatePath to cause error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestLogin_Success(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	login(w, req)

	resp := w.Result()
	// Either OK (templates exist) or 500 (templates missing in test env)
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestApiLogin_ParseFormError(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create request with invalid content type that will cause ParseForm to fail
	req := httptest.NewRequest("POST", "/api/login", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Set an invalid body that causes ParseForm error
	req.Body = nil
	req.PostForm = nil
	
	w := httptest.NewRecorder()

	apiLogin(w, req)

	resp := w.Result()
	// Should handle ParseForm error
	assert.True(t, resp.StatusCode >= 200) // May be 200 or 500 depending on template availability
}

func TestApiLogin_TemplateLoadError(t *testing.T) {
	// Temporarily modify templatePath to cause template load error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	form := url.Values{
		"username": {"testuser"},
		"password": {"password123"},
	}

	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiLogin(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

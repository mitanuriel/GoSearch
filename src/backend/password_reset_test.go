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

func TestCheckPasswordResetRequired_PasswordChanged(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock password_changed value check
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(true))

	result := checkPasswordResetRequired(1)

	assert.False(t, result, "Should return false when password already changed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckPasswordResetRequired_PasswordNotChanged(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock password_changed value check
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(false))

	result := checkPasswordResetRequired(2)

	assert.True(t, result, "Should return true when password not changed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckPasswordResetRequired_ColumnNotExists(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check - column doesn't exist
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock ALTER TABLE to add column
	mock.ExpectExec("ALTER TABLE users ADD COLUMN password_changed BOOLEAN DEFAULT TRUE").
		WillReturnResult(sqlmock.NewResult(0, 0))

	result := checkPasswordResetRequired(1)

	assert.True(t, result, "Should return true when column doesn't exist and is created")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResetPasswordHandler_NotLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()

	resetPasswordHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

func TestResetPasswordHandler_AlreadyChanged(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/reset-password", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock password_changed check
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(true))

	resetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestResetPasswordHandler_ShowForm(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/reset-password", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock password_changed check
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(false))

	// Mock username fetch
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	resetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApiResetPasswordHandler_NotLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	form := url.Values{
		"current_password": {"oldpass"},
		"new_password":     {"newpass123"},
		"confirm_password": {"newpass123"},
	}
	req := httptest.NewRequest("POST", "/api/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	apiResetPasswordHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

func TestApiResetPasswordHandler_PasswordMismatch(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("POST", "/api/reset-password", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	form := url.Values{
		"current_password": {"oldpass"},
		"new_password":     {"newpass123"},
		"confirm_password": {"different123"},
	}
	req2 := httptest.NewRequest("POST", "/api/reset-password", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock username fetch for error rendering
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	apiResetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApiResetPasswordHandler_ShortPassword(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("POST", "/api/reset-password", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	form := url.Values{
		"current_password": {"oldpass"},
		"new_password":     {"short"},
		"confirm_password": {"short"},
	}
	req2 := httptest.NewRequest("POST", "/api/reset-password", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock username fetch for error rendering
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	apiResetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApiResetPasswordHandler_WrongCurrentPassword(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("POST", "/api/reset-password", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	form := url.Values{
		"current_password": {"wrongpass"},
		"new_password":     {"newpass123"},
		"confirm_password": {"newpass123"},
	}
	req2 := httptest.NewRequest("POST", "/api/reset-password", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock password fetch
	hashedPassword, _ := hashPassword("correctpass")
	mock.ExpectQuery("SELECT password FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password"}).AddRow(hashedPassword))

	// Mock username fetch for error rendering
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	apiResetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPasswordResetMiddleware_SkipsPaths(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	testPaths := []string{
		"/reset-password",
		"/api/reset-password",
		"/login",
		"/api/login",
		"/register",
		"/api/register",
		"/api/logout",
		"/static/style.css",
	}

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := passwordResetMiddleware(nextHandler)

	for _, path := range testPaths {
		nextHandlerCalled = false
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.True(t, nextHandlerCalled, "Next handler should be called for path: %s", path)
	}
}

func TestPasswordResetMiddleware_RedirectsWhenResetRequired(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock column existence check
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock password_changed check - return false (reset required)
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(false))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Next handler should not be called when reset is required")
	})

	middleware := passwordResetMiddleware(nextHandler)
	middleware.ServeHTTP(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/reset-password", resp.Header.Get("Location"))
}

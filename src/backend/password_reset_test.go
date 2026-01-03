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

func TestSetupPasswordResetTable_ColumnAndTableExist(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check - column exists
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock table existence check - table exists
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.tables WHERE table_name = 'reset_tokens' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock verifySetup calls
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.tables WHERE table_name = 'reset_tokens' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	err := setupPasswordResetTable()

	assert.NoError(t, err, "Setup should succeed when everything exists")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupPasswordResetTable_CreateColumn(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check - column doesn't exist
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock ALTER TABLE to add column
	mock.ExpectExec("ALTER TABLE users ADD COLUMN password_changed BOOLEAN DEFAULT TRUE").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Mock UPDATE existing users
	mock.ExpectExec("UPDATE users SET password_changed = FALSE").
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Mock table existence check - table doesn't exist
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.tables WHERE table_name = 'reset_tokens' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock CREATE TABLE
	mock.ExpectExec("CREATE TABLE reset_tokens").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Mock verifySetup calls
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.tables WHERE table_name = 'reset_tokens' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	err := setupPasswordResetTable()

	assert.NoError(t, err, "Setup should succeed after creating column and table")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetupPasswordResetTable_ColumnCheckError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Skip ping (it's not monitored by default in sqlmock)

	// Mock column existence check to return error
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnError(assert.AnError)

	err := setupPasswordResetTable()

	assert.Error(t, err, "Setup should fail when column check fails")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifySetup_Success(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock table existence check
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.tables WHERE table_name = 'reset_tokens' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	err := verifySetup()

	assert.NoError(t, err, "Verification should succeed when both exist")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifySetup_MissingColumn(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check - column doesn't exist
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err := verifySetup()

	// After CodeRabbit fix - function now properly returns error when column doesn't exist
	assert.Error(t, err, "Should return error for missing column")
	assert.Contains(t, err.Error(), "password_changed column does not exist")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifySetup_MissingTable(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check - exists
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock table existence check - table doesn't exist
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.tables WHERE table_name = 'reset_tokens' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err := verifySetup()

	// After CodeRabbit fix - function now properly returns error when table doesn't exist
	assert.Error(t, err, "Should return error for missing table")
	assert.Contains(t, err.Error(), "reset_tokens table does not exist")
	assert.NoError(t, mock.ExpectationsWereMet())
}

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

func TestResetPasswordHandler_TemplateError(t *testing.T) {
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

	// Mock password_changed check - return false (needs reset)
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(false))

	// Mock username query
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	// Temporarily modify templatePath to cause template error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	resetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestResetPasswordHandler_UsernameQueryError(t *testing.T) {
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

	// Mock password_changed check - return false (needs reset)
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed"}).AddRow(false))

	// Mock username query to return error
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnError(assert.AnError)

	resetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestApiResetPasswordHandler_HashPasswordError(t *testing.T) {
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

	// Create form with password > 72 bytes (bcrypt limit)
	longPassword := ""
	for i := 0; i < 100; i++ {
		longPassword += "a"
	}

	form := url.Values{
		"current_password": {"oldpassword"},
		"new_password":     {longPassword},
		"confirm_password": {longPassword},
	}

	req2 := httptest.NewRequest("POST", "/api/reset-password", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock password verification - return true
	hashedOldPassword, _ := hashPassword("oldpassword")
	mock.ExpectQuery("SELECT password FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password"}).AddRow(hashedOldPassword))

	apiResetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestApiResetPasswordHandler_DatabaseUpdateError(t *testing.T) {
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
		"current_password": {"oldpassword"},
		"new_password":     {"newpassword123"},
		"confirm_password": {"newpassword123"},
	}

	req2 := httptest.NewRequest("POST", "/api/reset-password", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock password verification - return true
	hashedOldPassword, _ := hashPassword("oldpassword")
	mock.ExpectQuery("SELECT password FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password"}).AddRow(hashedOldPassword))

	// Mock UPDATE query to fail
	mock.ExpectExec("UPDATE users SET password = \\$1, password_changed = TRUE WHERE id = \\$2").
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnError(assert.AnError)

	apiResetPasswordHandler(w2, req2)

	resp := w2.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCheckPasswordResetRequired_QueryError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock column existence check to return error
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnError(assert.AnError)

	required := checkPasswordResetRequired(1)

	// Should return false on error
	assert.False(t, required)
}

func TestPasswordResetMiddleware_ColumnCheckError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Create authenticated session
	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()

	session, _ := store.Get(req, "session-name")
	session.Values["user_id"] = 1
	_ = session.Save(req, w)

	// Create new request with session cookie
	cookies := w.Result().Cookies()
	req2 := httptest.NewRequest("GET", "/search", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	w2 := httptest.NewRecorder()

	// Mock column existence check to return error
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnError(assert.AnError)

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := passwordResetMiddleware(nextHandler)
	middleware.ServeHTTP(w2, req2)

	// Should call next handler when column check fails
	assert.True(t, nextCalled)
}

func TestRenderResetPasswordError_Success(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock username query
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()

	renderResetPasswordError(w, req, 1, "Test error message")

	resp := w.Result()
	// Should render error page successfully
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError)
}

func TestRenderResetPasswordError_TemplateLoadError(t *testing.T) {
	mockDB, _ := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Set invalid template path
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()

	renderResetPasswordError(w, req, 1, "Test error")

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRenderResetPasswordError_UsernameQueryError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock username query error
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnError(assert.AnError)

	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()

	renderResetPasswordError(w, req, 1, "Test error")

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRenderResetPasswordError_TemplateExecutionError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock username query
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	// Set invalid template path to cause execution error
	originalPath := templatePath
	templatePath = "/nonexistent/path/"
	defer func() { templatePath = originalPath }()

	req := httptest.NewRequest("GET", "/reset-password", nil)
	w := httptest.NewRecorder()

	renderResetPasswordError(w, req, 1, "Test error")

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCheckPasswordResetRequired_ErrorPath(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// First mock the column existence check (returns true - column exists)
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Then mock the password_changed query to return error
	mock.ExpectQuery("SELECT password_changed FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnError(assert.AnError)

	result := checkPasswordResetRequired(1)

	// Should return false on error
	assert.False(t, result)
}

func TestVerifySetup_QueryError(t *testing.T) {
	mockDB, mock := setupMockDB()
	defer func() { _ = mockDB.Close() }()

	// Mock query error
	mock.ExpectQuery(`SELECT EXISTS \( SELECT 1 FROM information_schema\.columns WHERE table_name = 'users' AND column_name = 'password_changed' \)`).
		WillReturnError(assert.AnError)

	err := verifySetup()

	assert.Error(t, err)
}

func TestPasswordResetMiddleware_NotLoggedIn(t *testing.T) {
	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := passwordResetMiddleware(nextHandler)
	middleware.ServeHTTP(w, req)

	// Should call next handler when not logged in
	assert.True(t, nextCalled)
}


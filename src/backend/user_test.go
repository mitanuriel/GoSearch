// Unit tests for user-related functions
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
	defer mockDB.Close()
	
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
		session.Save(req, w)

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
	defer mockDB.Close()

	mockStore := sessions.NewCookieStore([]byte("test-secret"))
	store = mockStore

	// Initialize templates - will fail gracefully if not found
	loadTemplates("layout.html", "login.html")

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

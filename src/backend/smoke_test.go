//go:build smoke
// +build smoke

// Smoke tests for basic endpoint health checks
// Run with: go test -tags=smoke ./src/backend/...
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSmoke_RootEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for root endpoint, got %d", resp.StatusCode)
	}
}

func TestSmoke_AboutEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/about")
	if err != nil {
		t.Fatalf("GET /about failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /about, got %d", resp.StatusCode)
	}
}

func TestSmoke_LoginEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /login, got %d", resp.StatusCode)
	}
}

func TestSmoke_RegisterEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/register")
	if err != nil {
		t.Fatalf("GET /register failed: %v", err)
	}
	defer resp.Body.Close()

	// Should be OK (shows form) or redirect if already logged in
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Fatalf("Expected 200 OK or 302 Found for /register, got %d", resp.StatusCode)
	}
}

func TestSmoke_SearchEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/search?q=test")
	if err != nil {
		t.Fatalf("GET /search?q=test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /search, got %d", resp.StatusCode)
	}
}

func TestSmoke_WeatherEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/weather?city=Copenhagen")
	if err != nil {
		t.Fatalf("GET /weather failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /weather, got %d", resp.StatusCode)
	}
}

func TestSmoke_StaticCSSEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/static/style.css")
	if err != nil {
		t.Fatalf("GET /static/style.css failed: %v", err)
	}
	defer resp.Body.Close()

	// Static files might not be available in test environment, so accept 404 or 200
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected 200 OK or 404 Not Found for static file, got %d", resp.StatusCode)
	}
}

func TestSmoke_PrometheusMetricsEndpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /metrics, got %d", resp.StatusCode)
	}
}

func TestSmoke_404Endpoint(t *testing.T) {
	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found for nonexistent endpoint, got %d", resp.StatusCode)
	}
}

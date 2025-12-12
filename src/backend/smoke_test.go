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

func TestSmoke_HealthEndpoint(t *testing.T) {

	handler := setupRouter()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}
}

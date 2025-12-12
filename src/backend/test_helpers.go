// +build test integration smoke

// Test helper functions shared across different test types
package main

import (
	"github.com/gorilla/mux"
	"net/http"
)

// setupRouter creates an HTTP router for testing
// Used by integration and smoke tests
func setupRouter() http.Handler {
	if esClient == nil {
		initElasticsearch()
	}

	r := mux.NewRouter()
	r.HandleFunc("/", rootHandler).Methods("GET")
	r.HandleFunc("/about", aboutHandler).Methods("GET")
	r.HandleFunc("/api/weather", weatherHandler).Methods("GET")
	r.HandleFunc("/api/search", searchHandler).Methods("GET")
	r.HandleFunc("/api/login", apiLogin).Methods("POST")
	r.HandleFunc("/api/register", apiRegisterHandler).Methods("POST")
	r.HandleFunc("/reset-password", resetPasswordHandler).Methods("GET")
	r.HandleFunc("/api/reset-password", apiResetPasswordHandler).Methods("POST")
	return r
}

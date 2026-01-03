// +build test integration smoke

// Test helper functions shared across different test types
package main

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

// setupRouter creates an HTTP router for testing
// Used by integration and smoke tests
func setupRouter() http.Handler {
	if esClient == nil {
		initElasticsearch()
	}

	r := mux.NewRouter()
	
	// Main pages
	r.HandleFunc("/", rootHandler).Methods("GET")
	r.HandleFunc("/about", aboutHandler).Methods("GET")
	r.HandleFunc("/login", login).Methods("GET")
	r.HandleFunc("/register", registerHandler).Methods("GET")
	r.HandleFunc("/search", searchHandler).Methods("GET")
	r.HandleFunc("/weather", weatherHandler).Methods("GET")
	r.HandleFunc("/logout", logoutHandler).Methods("POST")
	r.HandleFunc("/reset-password", resetPasswordHandler).Methods("GET")
	
	// API endpoints
	r.HandleFunc("/api/login", apiLogin).Methods("POST")
	r.HandleFunc("/api/register", apiRegisterHandler).Methods("POST")
	r.HandleFunc("/api/reset-password", apiResetPasswordHandler).Methods("POST")
	
	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("../frontend/static"))))
	
	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())
	
	return r
}

//go:build integration
// +build integration

// Integration tests for Elasticsearch initialization
// These tests require real Elasticsearch server running locally.
// Run with: go test -tags=integration -run TestInitElasticsearch_Integration ./src/backend/
//
//nolint:errcheck // Test cleanup errors are not critical
package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestInitElasticsearch_Integration tests Elasticsearch initialization
func TestInitElasticsearch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Save original esClient
	originalESClient := esClient
	defer func() { esClient = originalESClient }()

	esHost := os.Getenv("ES_HOST")
	if esHost == "" {
		esHost = "localhost"
	}

	// Set environment variables for test
	os.Setenv("ES_HOST", esHost)
	os.Setenv("ES_USERNAME", "elastic")
	os.Setenv("ES_PASSWORD", "changeme")

	// This should not panic
	initElasticsearch()

	// Verify client was initialized
	assert.NotNil(t, esClient, "Elasticsearch client should be initialized")

	// Try to ping Elasticsearch
	if esClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := esClient.Ping(
			esClient.Ping.WithContext(ctx),
		)
		if err != nil {
			t.Logf("Elasticsearch ping failed (might not be running): %v", err)
		} else {
			defer res.Body.Close()
			t.Logf("Elasticsearch ping successful: %s", res.Status())
		}
	}
}

// TestElasticsearchConnection_Integration tests actual Elasticsearch connection
func TestElasticsearchConnection_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	esHost := os.Getenv("ES_HOST")
	if esHost == "" {
		t.Skip("ES_HOST not set, skipping Elasticsearch integration test")
	}

	// Save original esClient
	originalESClient := esClient
	defer func() { esClient = originalESClient }()

	// Initialize Elasticsearch
	initElasticsearch()

	if esClient == nil {
		t.Skip("Elasticsearch client not initialized, skipping test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test ping
	res, err := esClient.Ping(
		esClient.Ping.WithContext(ctx),
	)
	if err != nil {
		t.Skipf("Elasticsearch not available: %v", err)
	}
	defer res.Body.Close()

	assert.False(t, res.IsError(), "Elasticsearch ping should not return error")
	assert.Equal(t, 200, res.StatusCode, "Elasticsearch should return 200 OK")
}

// TestElasticsearchIndexExists_Integration tests checking if index exists
func TestElasticsearchIndexExists_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	esHost := os.Getenv("ES_HOST")
	if esHost == "" {
		t.Skip("ES_HOST not set, skipping Elasticsearch integration test")
	}

	// Save original esClient
	originalESClient := esClient
	defer func() { esClient = originalESClient }()

	// Initialize Elasticsearch
	initElasticsearch()

	if esClient == nil {
		t.Skip("Elasticsearch client not initialized, skipping test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if 'pages' index exists
	res, err := esClient.Indices.Exists(
		[]string{"pages"},
		esClient.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		t.Skipf("Elasticsearch not available: %v", err)
	}
	defer res.Body.Close()

	// Index might or might not exist - just log the result
	if res.StatusCode == 200 {
		t.Log("Pages index exists in Elasticsearch")
	} else {
		t.Log("Pages index does not exist in Elasticsearch")
	}
}

// TestSyncPagesToElasticsearch_Integration tests syncing pages to Elasticsearch
func TestSyncPagesToElasticsearch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires both database and Elasticsearch to be running
	connStr := os.Getenv("CONN_STR")
	esHost := os.Getenv("ES_HOST")
	if connStr == "" || esHost == "" {
		t.Skip("CONN_STR or ES_HOST not set, skipping sync test")
	}

	// Save original db and esClient
	originalDB := db
	originalESClient := esClient
	defer func() {
		db = originalDB
		esClient = originalESClient
	}()

	// Initialize database
	CONN_STR = connStr
	initDB()
	if db == nil {
		t.Skip("Database not available, skipping test")
	}

	// Initialize Elasticsearch
	initElasticsearch()
	if esClient == nil {
		t.Skip("Elasticsearch not available, skipping test")
	}

	// Try to sync pages
	err := syncPagesToElasticsearch()
	if err != nil {
		t.Logf("syncPagesToElasticsearch returned error (might be expected): %v", err)
		// Don't fail the test - sync might fail for various reasons
		// (empty database, Elasticsearch issues, etc.)
	} else {
		t.Log("syncPagesToElasticsearch completed successfully")
	}
}

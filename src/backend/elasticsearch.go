package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

func initElasticsearch() {
	var err error
	maxRetries := 10
	retryDelay := time.Second * 5
	esHost := os.Getenv("ES_HOST")
	if esHost == "" {
		esHost = "localhost"
	}
	esPassword := os.Getenv("ES_PASSWORD")
	if esPassword == "" {
		esPassword = "changeme"
	}
	esUsername := os.Getenv("ES_USERNAME")
	if esUsername == "" {
		esUsername = "elastic"
	}

	for i := 0; i < maxRetries; i++ {
		// Try both HTTPS and HTTP connections
		configs := []elasticsearch.Config{
			// Try HTTP first
			{
				Addresses: []string{fmt.Sprintf("http://%s:9200", esHost)},
				Username:  esUsername,
				Password:  esPassword,
			},
			// Try HTTPS as fallback
			{
				Addresses: []string{fmt.Sprintf("https://%s:9200", esHost)},
				Username:  esUsername,
				Password:  esPassword,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			},
		}

		// Try each config until one works
		for _, config := range configs {
			esClient, err = elasticsearch.NewClient(config)
			if err != nil {
				log.Printf("Error creating Elasticsearch client with config %v: %s", config.Addresses, err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			res, err := esClient.Info(esClient.Info.WithContext(ctx))
			cancel()

			if err == nil {
				defer res.Body.Close()
				log.Printf("Successfully connected to Elasticsearch via %s", config.Addresses[0])

				// Check if 'pages' index exists
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				existsRes, err := esClient.Indices.Exists(
					[]string{"pages"},
					esClient.Indices.Exists.WithContext(ctx),
				)
				cancel()

				if err != nil {
					log.Printf("Error checking if index exists: %v", err)
				} else {
					if existsRes.StatusCode == 404 {
						log.Println("Creating 'pages' index with proper mappings")

						// Define index mappings with correct field types
						mappings := `{
                            "mappings": {
                                "properties": {
                                    "title": { "type": "text" },
                                    "url": { "type": "keyword" },
                                    "content": { "type": "text" },
                                    "language": { "type": "keyword" },
                                    "last_updated": { "type": "date" }
                                }
                            }
                        }`

						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						createRes, err := esClient.Indices.Create(
							"pages",
							esClient.Indices.Create.WithBody(strings.NewReader(mappings)),
							esClient.Indices.Create.WithContext(ctx),
						)
						cancel()

						if err != nil {
							log.Printf("Error creating index: %v", err)
						} else if createRes.IsError() {
							log.Printf("Error response when creating index: %s", createRes.String())
						} else {
							log.Println("Created 'pages' index successfully")
						}
					} else {
						log.Println("'pages' index already exists")
					}
				}

				return
			}

			log.Printf("Error connecting to Elasticsearch via %s: %v", config.Addresses[0], err)
		}

		log.Printf("Could not connect to Elasticsearch (attempt %d/%d). Retrying in %v...",
			i+1, maxRetries, retryDelay)
		time.Sleep(retryDelay)
	}

	// Changed from log.Fatalf to log.Printf - allows app to continue
	log.Printf("Warning: Failed to connect to Elasticsearch after %d attempts", maxRetries)
	log.Println("Application will continue with PostgreSQL search fallback")
	esClient = nil // Ensure esClient is nil so fallback is used
}

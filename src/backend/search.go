package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	//Henter search-query fra URL-parameteren.
	log.Println("Search handler called")

	queryParam := strings.TrimSpace(r.URL.Query().Get("q"))
	if queryParam == "" {
		http.Error(w, "No search query provided", http.StatusBadRequest)
		return
	}
	//TO LOG THE QUERY//
	log.Printf("Search query: %q from %s", queryParam, r.RemoteAddr)
	searchLogger.Printf("query=%q from=%s", queryParam, r.RemoteAddr)

	//Nuild search against Elasticsearch
	pages, err := searchPagesInEs(queryParam)
	if err != nil {
		log.Printf("Error searching Elasticsearch: %v", err)
		http.Error(w, "Error during search", http.StatusInternalServerError)
		return
	}

	// Build search results from Elasticsearch response
	var searchResults []map[string]string
	for _, page := range pages {
		searchResults = append(searchResults, map[string]string{
			"title":       page.Title,
			"url":         page.URL,
			"description": page.Content,
		})
	}

	tmpl, err := template.ParseFiles(templatePath+"layout.html", templatePath+"search.html")
	if err != nil {
		log.Printf("Error parsing search templates: %v", err)
		http.Error(w, "Error loading search template", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Query":   queryParam,
		"Results": searchResults,
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error executing search template: %v", err)
		http.Error(w, "Error rendering search results", http.StatusInternalServerError)
	}
}

func searchPagesInEs(query string) ([]Page, error) {
	///// TESTS FALLBACK ///////////
	if esClient == nil {
		// Simple DB search for test mode
		var pages []Page
		sqlStmt := "SELECT title, url, content FROM pages WHERE content LIKE ?"
		likeQ := "%" + query + "%"
		rows, err := db.Query(sqlStmt, likeQ)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var p Page
			if err := rows.Scan(&p.Title, &p.URL, &p.Content); err != nil {
				continue
			}
			pages = append(pages, p)
		}
		return pages, nil
	}
	/////// PRODUCTION: real Elasticsearch search ───────────────────────────
	var pages []Page

	searchBody := strings.NewReader(fmt.Sprintf(`{
		"query": {
			"multi_match": {
				"query": "%s",
				"fields": ["title^3", "url^2", "content"]
			}
		}
	}`, query))

	res, err := esClient.Search(
		esClient.Search.WithContext(context.Background()),
		esClient.Search.WithIndex("pages"),
		esClient.Search.WithBody(searchBody),
		esClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return pages, err
	}
	defer func() { _ = res.Body.Close() }()

	var r struct {
		Hits struct {
			Hits []struct {
				Source Page `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return pages, err
	}

	for _, hit := range r.Hits.Hits {
		pages = append(pages, hit.Source)
	}

	return pages, nil
}

func syncPagesToElasticsearch() error {
	// Først, slet indekset hvis det eksisterer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	existsRes, err := esClient.Indices.Exists(
		[]string{"pages"},
		esClient.Indices.Exists.WithContext(ctx),
	)
	cancel()

	if err != nil {
		return fmt.Errorf("error checking if index exists: %w", err)
	}

	if existsRes.StatusCode == 200 {
		log.Println("Index 'pages' already exists - removing and rebuilding")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		deleteRes, err := esClient.Indices.Delete(
			[]string{"pages"},
			esClient.Indices.Delete.WithContext(ctx),
		)
		cancel()

		if err != nil {
			return fmt.Errorf("error deleting index: %w", err)
		}

		if deleteRes.IsError() {
			return fmt.Errorf("error response when deleting index: %s", deleteRes.String())
		}
	}

	// Opret indekset med korrekte mappings
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

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	createRes, err := esClient.Indices.Create(
		"pages",
		esClient.Indices.Create.WithBody(strings.NewReader(mappings)),
		esClient.Indices.Create.WithContext(ctx),
	)
	cancel()

	if err != nil {
		return fmt.Errorf("error creating index: %w", err)
	}

	if createRes.IsError() {
		return fmt.Errorf("error response when creating index: %s", createRes.String())
	}

	// Hent og indekser alle sider fra databasen
	rows, err := db.Query("SELECT title, url, content FROM pages")
	if err != nil {
		return fmt.Errorf("error querying pages from DB: %w", err)
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var title, url, content string
		if err := rows.Scan(&title, &url, &content); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		// Opret dokument med de rigtige feltnavne
		docMap := map[string]interface{}{
			"title":        title,
			"url":          url,
			"content":      content,
			"language":     "",
			"last_updated": time.Now().Format(time.RFC3339),
		}

		doc, err := json.Marshal(docMap)
		if err != nil {
			log.Printf("Error marshaling page: %v", err)
			continue
		}

		// Indekser dokumentet med eget generert id.
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		indexRes, err := esClient.Index(
			"pages",
			strings.NewReader(string(doc)),
			esClient.Index.WithRefresh("true"),
			esClient.Index.WithContext(ctx),
		)
		cancel()

		if err != nil {
			log.Printf("Error indexing %s: %v", url, err)
			continue
		}

		if indexRes.IsError() {
			log.Printf("Error response when indexing %s: %s", url, indexRes.String())
			continue
		}

		log.Printf("Indexed page: %s", url)
		count++
	}

	log.Printf("Synced %d pages to Elasticsearch", count)
	return nil
}

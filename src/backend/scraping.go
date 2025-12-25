package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func extractSearchTerms(logPath string) []string {
	file, err := os.Open(logPath)
	if err != nil {
		log.Printf("Could not open log: %v", err)
		return nil
	}
	defer func() { _ = file.Close() }()

	re := regexp.MustCompile(`query="([^"]+)"`)
	termsMap := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			term := strings.ToLower(strings.TrimSpace(match[1]))
			termsMap[term] = true
			fmt.Printf("Extracted search term: %s\n", term)
		}
	}

	var terms []string
	for k := range termsMap {
		terms = append(terms, k)
	}
	return terms
}

func alreadyProcessed(term string) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM processed_searches WHERE search_term = $1)", term).Scan(&exists)
	if err != nil {
		log.Printf("Error checking processed term: %v", err)
		return false // Fallback: antag at den ikke er behandlet
	}
	return exists
}

func markAsProcessed(term string) {
	_, err := db.Exec("INSERT INTO processed_searches (search_term) VALUES ($1) ON CONFLICT DO NOTHING", term)
	if err != nil {
		log.Printf("Error marking term as processed: %v", err)
	}
}

func StartScraping(logPath string) {
	searchTerms := extractSearchTerms(logPath)
	if len(searchTerms) == 0 {
		fmt.Println("No search terms found.")
		return
	}

	for _, term := range searchTerms {
		if alreadyProcessed(term) {
			fmt.Printf("Skipping already processed term: %s\n", term)
			continue
		}

		page, lang, err := tryScrapeInLanguages(term, []string{"da", "en"})
		if err != nil {
			log.Printf("Failed to scrape any language for term '%s': %v", term, err)
			continue
		}

		err = savePageToDBWithLang(page, lang)
		if err != nil {
			log.Printf("Error saving page to DB: %v", err)
			continue
		}

		markAsProcessed(term)
	}
}

func tryScrapeInLanguages(term string, langs []string) (Page, string, error) {
	for _, lang := range langs {
		url := buildWikipediaURL(term, lang)
		fmt.Printf("Trying to scrape: %s\n", url)
		page, err := scrapeWikipedia(url, lang)
		if err == nil && page.Title != "" {
			return page, lang, nil
		}
		log.Printf("Failed scraping %s (%s): %v", term, lang, err)
	}
	return Page{}, "", fmt.Errorf("no valid Wikipedia page found for term '%s'", term)
}

func buildWikipediaURL(term, lang string) string {
	term = strings.ReplaceAll(term, " ", "_")
	c := cases.Title(language.Und)
	return fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, c.String(term))
}

func scrapeWikipedia(url string, lang string) (Page, error) {
	c := colly.NewCollector(
		colly.AllowedDomains(fmt.Sprintf("%s.wikipedia.org", lang)),
	)

	page := Page{URL: url, Language: lang}
	var statusCode int

	c.OnResponse(func(r *colly.Response) {
		statusCode = r.StatusCode
	})

	c.OnHTML("#firstHeading", func(e *colly.HTMLElement) {
		page.Title = e.Text
	})

	c.OnHTML("div.mw-parser-output", func(e *colly.HTMLElement) {
		text := ""
		e.ForEach("p", func(_ int, el *colly.HTMLElement) {
			text += el.Text + "\n"
		})
		page.Content = text
	})

	err := c.Visit(url)
	if err != nil {
		return page, err
	}

	if statusCode == 404 {
		return page, fmt.Errorf("page not found (404)")
	}

	return page, nil
}

func savePageToDBWithLang(page Page, lang string) error {
	if page.Title == "" || page.URL == "" || page.Content == "" {
		return fmt.Errorf("invalid page data")
	}

	_, err := db.Exec(`
		INSERT INTO pages (url, title, content, language, last_updated)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (url) DO UPDATE
		SET title = EXCLUDED.title,
		    content = EXCLUDED.content,
		    language = EXCLUDED.language,
		    last_updated = NOW()
	`, page.URL, page.Title, page.Content, lang)
	if err != nil {
		return fmt.Errorf("error inserting or updating page: %v", err)
	}

	log.Printf("Saved page to DB [%s]: %s", lang, page.Title)
	return nil
}

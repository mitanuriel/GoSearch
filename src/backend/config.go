package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
)

var staticPath = "static"

var searchLogger = log.New(os.Stdout, "SEARCH: ", log.LstdFlags)

var CONN_STR string

var templatePath string

var db *sql.DB

var esClient *elasticsearch.Client

var store *sessions.CookieStore

func init() {

	if err := godotenv.Load("../../.env.local"); err != nil {
		// If .env.local doesn't exist, try regular .env
		if godotenv.Load() != nil {
			log.Println("No .env files found. Using environment variables.")
		}
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost != "" && dbUser != "" && dbName != "" {
		if dbPort == "" {
			dbPort = "5432" // Default PostgreSQL port
		}
		CONN_STR = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPassword, dbName)
	} else {
		// Fall back to the full connection string from environment
		CONN_STR = os.Getenv("CONN_STR")
		if CONN_STR == "" {
			log.Println("Warning: No database connection details found in environment variables")
			log.Println("Please set DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME or CONN_STR")
		}
	}

	templatePath = os.Getenv("TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "../frontend/templates/"
	}

	staticPath = os.Getenv("STATIC_PATH")
	if staticPath == "" {
		staticPath = "../frontend/static/"
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" || sessionSecret == "Very-secret-key" {
		log.Fatal("SESSION_SECRET is not set or insecure. Please set a strong SESSION_SECRET in your environment.")
	}

	store = sessions.NewCookieStore([]byte(sessionSecret))

}

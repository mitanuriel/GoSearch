package main

import (
	"html/template"
	"net/http"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {

	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"]

	data := map[string]any{
		"Title":        "Home",
		"UserLoggedIn": ok && userID != nil,
	}

	tmpl, err := template.ParseFiles(templatePath+"layout.html", templatePath+"index.html")
	if err != nil {
		handleInternalError(w, r, err, "Failed to load homepage templates")
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		handleInternalError(w, r, err, "Failed to render homepage")
	}
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	userID, ok := session.Values["user_id"]

	data := map[string]interface{}{
		"UserLoggedIn": ok && userID != nil,
	}

	tmpl, err := template.ParseFiles(templatePath+"layout.html", templatePath+"about.html")
	if err != nil {
		handleInternalError(w, r, err, "Failed to load about page templates")
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		handleInternalError(w, r, err, "Failed to render about page")
	}
}

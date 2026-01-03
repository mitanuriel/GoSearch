package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/csrf"
)

func getTemplates() (*template.Template, error) {
	return template.ParseFiles(templatePath+"layout.html", templatePath+"login.html")
}

func loadTemplates(files ...string) (*template.Template, error) {
	var paths []string
	for _, file := range files {
		paths = append(paths, templatePath+file)
	}

	return template.ParseFiles(paths...)
}

func login(w http.ResponseWriter, r *http.Request) {

	tmpl, err := getTemplates()

	if err != nil {
		handleInternalError(w, r, err, "Failed to load login templates")
		return
	}

	data := PageData{
		Title:        "Log in",
		Template:     "login",
		UserLoggedIn: userIsLoggedIn(r),
		CSRFToken:    csrf.Token(r),
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		handleInternalError(w, r, err, "Failed to render login page")
	}

}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session-name")
	if err == nil {
		// Get user ID for session tracking before we delete it
		if userID, ok := session.Values["user_id"]; ok && userID != nil {
			sessionID := fmt.Sprintf("%v", userID)
			removeActiveSession(sessionID, "authenticated")
		}

		//Delete session data
		session.Options.MaxAge = -1 // Expire the session
		delete(session.Values, "user_id")

		if err := session.Save(r, w); err != nil {
			handleInternalError(w, r, err, "Failed to save session during logout")
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func apiLogin(w http.ResponseWriter, r *http.Request) {

	tmpl, err := loadTemplates("layout.html", "login.html")
	if err != nil {
		handleInternalError(w, r, err, "Failed to load login templates")
		return
	}

	err = r.ParseForm()
	if err != nil {
		data := PageData{
			Title:        "Log in",
			Template:     "login.html",
			Error:        "Invalid username or password",
			UserLoggedIn: false,
		}
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			log.Printf("Template execution error: %v", err)
			return
		}
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Checks that username and password are not empty strings
	if username == "" || password == "" {

		data := PageData{
			Title:        "Log in",
			Template:     "login.html",
			Error:        "Username and password cannot be empty",
			UserLoggedIn: false,
		}
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, "Failed to render error page", http.StatusInternalServerError)
		}
		return
	}

	var user User

	// Finds the user in the db based on the username
	err = db.QueryRow("SELECT id, username, password FROM users WHERE username = $1", username).Scan(&user.ID, &user.Username, &user.Password)

	// If the username is not found in th db or password is incorrect
	if err != nil {
		data := PageData{
			Title:        "Log in",
			Template:     "login.html",
			Error:        "Incorrect username or password",
			UserLoggedIn: false,
		}
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			log.Printf("Template execution error: %v", err)
			return
		}
		return
	}

	if !validatePassword(user.Password, password) {
		data := PageData{
			Title:        "Log in",
			Template:     "login.html",
			Error:        "Incorrect username or password",
			UserLoggedIn: false,
		}

		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			log.Printf("Template execution error: %v", err)
			return
		}
		return
	}

	session, err := store.Get(r, "session-name")
	if err != nil {
		data := PageData{
			Title:        "Log in",
			Template:     "login.html",
			Error:        "Session error",
			UserLoggedIn: false,
		}
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, "Failed to render error page", http.StatusInternalServerError)
		}
		return
	}

	session.Values["user_id"] = user.ID
	if err := session.Save(r, w); err != nil {
		data := PageData{
			Title:        "Log in",
			Template:     "login.html",
			Error:        "Session save error",
			UserLoggedIn: false,
		}
		w.WriteHeader(http.StatusInternalServerError)
		if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, "Failed to render error page", http.StatusInternalServerError)
		}
		return
	}
	userID := fmt.Sprintf("%v", user.ID)
	incrementUserSessionsTotal("authenticated")
	trackActiveSession(userID, "authenticated")

	http.Redirect(w, r, "/", http.StatusSeeOther)

}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if userIsLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	tmpl, err := template.ParseFiles(templatePath+"layout.html", templatePath+"register.html")
	if err != nil {
		handleInternalError(w, r, err, "Failed to load register templates")
		return
	}

	data := PageData{
		Title:        "Register",
		Error:        "",
		UserLoggedIn: false,
		Template:     "register.html",
		CSRFToken:    csrf.Token(r),
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		handleInternalError(w, r, err, "Failed to render register page")
	}
}

// Håndterer registreringsprocessen
func apiRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if userIsLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	err := r.ParseForm()
	if err != nil {
		handleBadRequest(w, r, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")
	password2 := r.FormValue("password2")

	if username == "" {
		handleBadRequest(w, r, "You have to enter a username")
		return
	}
	if email == "" || !isValidEmail(email) {
		handleBadRequest(w, r, "You have to enter a valid email address")
		return
	}
	if password == "" {
		handleBadRequest(w, r, "You have to enter a password")
		return
	}
	if password != password2 {
		handleBadRequest(w, r, "The two passwords do not match")
		return
	}

	// Tjek for eksisterende brugernavn og email
	usernameTaken, emailTaken := userExists(username, email)
	if usernameTaken {
		handleBadRequest(w, r, "The username is already taken")
		return
	}

	if emailTaken {
		handleBadRequest(w, r, "A user with this email already exists")
		return
	}

	// Hash passwordet
	hashedPassword, err := hashPassword(password)
	if err != nil {
		handleInternalError(w, r, err, "Error hashing password")
		return
	}

	var userID int
	err = db.QueryRow("INSERT INTO users (username, email, password, password_changed) VALUES ($1, $2, $3, TRUE) RETURNING id",
		username, email, hashedPassword).Scan(&userID)

	if err != nil {
		handleInternalError(w, r, err, "Database error during user registration")
		return
	}

	// Function to update number of users along with when they were created
	incrementNewUserCounter()

	// Create session and log the user in
	session, err := store.Get(r, "session-name")
	if err != nil {
		handleInternalError(w, r, err, "Session error during registration")
		return
	}

	session.Values["user_id"] = int(userID)
	if err := session.Save(r, w); err != nil {
		handleInternalError(w, r, err, "Session save error during registration")
		return
	}

	sessionID := fmt.Sprintf("%v", userID)
	incrementUserSessionsTotal("authenticated")
	trackActiveSession(sessionID, "authenticated")

	// Omdiriger til forsiden (bruger er nu logget ind)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

package handlers

import (
	"net/http"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/middleware"
)

func LoginPage(cfg config.Config, sessionManager *middleware.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionManager.VerifySessionFromRequest(r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		ServeSPA(cfg).ServeHTTP(w, r)
	}
}

func LoginSubmit(cfg config.Config, sessionManager *middleware.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}

		password := r.FormValue("password")
		if !sessionManager.VerifyPassword(password, cfg.AppPassword) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Invalid password"))
			return
		}

		if err := sessionManager.SetSessionCookie(w, r); err != nil {
			http.Error(w, "Could not create session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func Logout(sessionManager *middleware.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionManager.ClearSessionCookie(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

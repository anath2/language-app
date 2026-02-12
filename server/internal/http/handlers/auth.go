package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/middleware"
)

func Login(cfg config.Config, sessionManager *middleware.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
			return
		}

		if !sessionManager.VerifyPassword(payload.Password, cfg.AppPassword) {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid password"})
			return
		}

		if err := sessionManager.SetSessionCookie(w, r); err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Could not create session"})
			return
		}

		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func Logout(sessionManager *middleware.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionManager.ClearSessionCookie(w, r)
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

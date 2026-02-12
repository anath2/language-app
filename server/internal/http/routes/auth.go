package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/anath2/language-app/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func RegisterAuthRoutes(r chi.Router, cfg config.Config, sessionManager *middleware.SessionManager) {
	r.Method(http.MethodPost, "/api/auth/login", handlers.Login(cfg, sessionManager))
	r.Method(http.MethodPost, "/api/auth/logout", handlers.Logout(sessionManager))
}

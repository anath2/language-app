package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/anath2/language-app/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func RegisterAuthRoutes(r chi.Router, cfg config.Config, sessionManager *middleware.SessionManager) {
	r.Method(http.MethodGet, "/login", handlers.LoginPage(cfg, sessionManager))
	r.Method(http.MethodPost, "/login", handlers.LoginSubmit(cfg, sessionManager))
	r.Method(http.MethodPost, "/logout", handlers.Logout(sessionManager))
	r.Method(http.MethodGet, "/", handlers.ServeSPA(cfg))
	r.Method(http.MethodGet, "/translations", handlers.ServeSPA(cfg))
}

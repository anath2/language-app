package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterAdminRoutes(r chi.Router, cfg config.Config) {
	r.Method(http.MethodGet, "/admin", handlers.AdminPage(cfg))
	r.Method(http.MethodGet, "/admin/progress/export", http.HandlerFunc(handlers.ExportProgress))
	r.Method(http.MethodPost, "/admin/progress/import", http.HandlerFunc(handlers.ImportProgress))
	r.Method(http.MethodGet, "/admin/api/profile", http.HandlerFunc(handlers.GetProfile))
	r.Method(http.MethodPost, "/admin/api/profile", http.HandlerFunc(handlers.UpdateProfile))
}

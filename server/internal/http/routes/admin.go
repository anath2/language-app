package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterAdminRoutes(r chi.Router) {
	r.Method(http.MethodGet, "/api/admin/progress/export", http.HandlerFunc(handlers.ExportProgress))
	r.Method(http.MethodPost, "/api/admin/progress/import", http.HandlerFunc(handlers.ImportProgress))
	r.Method(http.MethodGet, "/api/admin/profile", http.HandlerFunc(handlers.GetProfile))
	r.Method(http.MethodPost, "/api/admin/profile", http.HandlerFunc(handlers.UpdateProfile))
}

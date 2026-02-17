package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterDiscoveryRoutes(r chi.Router) {
	r.Route("/api/discovery", func(api chi.Router) {
		api.Method(http.MethodGet, "/preferences", http.HandlerFunc(handlers.ListDiscoveryPreferences))
		api.Method(http.MethodPost, "/preferences", http.HandlerFunc(handlers.SaveDiscoveryPreference))
		api.Method(http.MethodDelete, "/preferences/{id}", http.HandlerFunc(handlers.DeleteDiscoveryPreference))

		api.Method(http.MethodGet, "/articles", http.HandlerFunc(handlers.ListDiscoveryArticles))
		api.Method(http.MethodGet, "/articles/{id}", http.HandlerFunc(handlers.GetDiscoveryArticle))
		api.Method(http.MethodPost, "/articles/{id}/dismiss", http.HandlerFunc(handlers.DismissDiscoveryArticle))
		api.Method(http.MethodPost, "/articles/{id}/import", http.HandlerFunc(handlers.ImportDiscoveryArticle))

		api.Method(http.MethodPost, "/run", http.HandlerFunc(handlers.TriggerDiscoveryRun))
	})
}

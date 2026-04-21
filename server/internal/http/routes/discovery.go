package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterDiscoveryRoutes(r chi.Router) {
	r.Method(http.MethodGet, "/api/discovery/status", http.HandlerFunc(handlers.DiscoveryStatus))
}

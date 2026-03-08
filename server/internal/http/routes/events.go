package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterEventRoutes(r chi.Router) {
	r.Method(http.MethodPost, "/api/events", http.HandlerFunc(handlers.CreateEvent))
}

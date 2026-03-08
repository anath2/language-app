package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterTextRoutes(r chi.Router) {
	r.Method(http.MethodPost, "/api/texts", http.HandlerFunc(handlers.CreateText))
	r.Method(http.MethodGet, "/api/texts/{text_id}", http.HandlerFunc(handlers.GetText))
}

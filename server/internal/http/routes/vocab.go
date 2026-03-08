package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterVocabRoutes(r chi.Router) {
	r.Method(http.MethodPost, "/api/vocab/save", http.HandlerFunc(handlers.SaveVocab))
	r.Method(http.MethodPost, "/api/vocab/status", http.HandlerFunc(handlers.UpdateVocabStatus))
	r.Method(http.MethodPost, "/api/vocab/lookup", http.HandlerFunc(handlers.RecordLookup))
	r.Method(http.MethodGet, "/api/vocab/srs-info", http.HandlerFunc(handlers.GetVocabSRSInfo))
}

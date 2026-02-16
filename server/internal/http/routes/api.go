package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterAPIRoutes(r chi.Router) {
	r.Route("/api", func(api chi.Router) {
		api.Method(http.MethodPost, "/texts", http.HandlerFunc(handlers.CreateText))
		api.Method(http.MethodGet, "/texts/{text_id}", http.HandlerFunc(handlers.GetText))
		api.Method(http.MethodPost, "/events", http.HandlerFunc(handlers.CreateEvent))
		api.Method(http.MethodPost, "/vocab/save", http.HandlerFunc(handlers.SaveVocab))
		api.Method(http.MethodPost, "/vocab/status", http.HandlerFunc(handlers.UpdateVocabStatus))
		api.Method(http.MethodPost, "/vocab/lookup", http.HandlerFunc(handlers.RecordLookup))
		api.Method(http.MethodGet, "/vocab/srs-info", http.HandlerFunc(handlers.GetVocabSRSInfo))
		api.Method(http.MethodPost, "/review/answer", http.HandlerFunc(handlers.RecordReviewAnswer))
		api.Method(http.MethodGet, "/review/words/queue", http.HandlerFunc(handlers.GetReviewQueue))
		api.Method(http.MethodGet, "/review/words/count", http.HandlerFunc(handlers.GetReviewCount))
		api.Method(http.MethodGet, "/review/characters/queue", http.HandlerFunc(handlers.GetCharacterReviewQueue))
		api.Method(http.MethodGet, "/review/characters/count", http.HandlerFunc(handlers.GetCharacterReviewCount))
		api.Method(http.MethodPost, "/segments/translate-batch", http.HandlerFunc(handlers.TranslateBatch))
	})
}

package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterTranslationRoutes(r chi.Router) {
	r.Method(http.MethodPost, "/api/translations", http.HandlerFunc(handlers.CreateTranslation))
	r.Method(http.MethodGet, "/api/translations", http.HandlerFunc(handlers.ListTranslations))
	r.Method(http.MethodGet, "/api/translations/{translation_id}", http.HandlerFunc(handlers.GetTranslation))
	r.Method(http.MethodGet, "/api/translations/{translation_id}/status", http.HandlerFunc(handlers.GetTranslationStatus))
	r.Method(http.MethodPatch, "/api/translations/{translation_id}", http.HandlerFunc(handlers.UpdateTranslation))
	r.Method(http.MethodDelete, "/api/translations/{translation_id}", http.HandlerFunc(handlers.DeleteTranslation))
	r.Method(http.MethodGet, "/api/translations/{translation_id}/stream", http.HandlerFunc(handlers.TranslationStream))
	r.Method(http.MethodPost, "/api/translations/segments/batch", http.HandlerFunc(handlers.TranslateBatch))
	r.Method(http.MethodPost, "/api/translations/{translation_id}/chat/new", http.HandlerFunc(handlers.CreateChatMessage))
	r.Method(http.MethodGet, "/api/translations/{translation_id}/chat/list", http.HandlerFunc(handlers.ListChatMessages))
	r.Method(http.MethodPost, "/api/translations/{translation_id}/chat/clear", http.HandlerFunc(handlers.ClearChatMessages))
	r.Method(http.MethodPost, "/api/translations/{translation_id}/chat/messages/{message_id}/accept", http.HandlerFunc(handlers.AcceptReviewCard))
	r.Method(http.MethodPost, "/api/translations/{translation_id}/chat/messages/{message_id}/reject", http.HandlerFunc(handlers.RejectReviewCard))
}

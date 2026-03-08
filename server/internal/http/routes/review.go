package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterReviewRoutes(r chi.Router) {
	r.Method(http.MethodPost, "/api/review/answer", http.HandlerFunc(handlers.RecordReviewAnswer))
	r.Method(http.MethodGet, "/api/review/words/queue", http.HandlerFunc(handlers.GetReviewQueue))
	r.Method(http.MethodGet, "/api/review/words/count", http.HandlerFunc(handlers.GetReviewCount))
	r.Method(http.MethodGet, "/api/review/characters/queue", http.HandlerFunc(handlers.GetCharacterReviewQueue))
	r.Method(http.MethodGet, "/api/review/characters/count", http.HandlerFunc(handlers.GetCharacterReviewCount))
}

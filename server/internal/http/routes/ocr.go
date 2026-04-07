package routes

import (
	"net/http"

	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterOCRRoutes(r chi.Router) {
	r.Method(http.MethodPost, "/api/ocr/extract-text", http.HandlerFunc(handlers.ExtractText))
}

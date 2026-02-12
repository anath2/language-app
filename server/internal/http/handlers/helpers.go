package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func NotImplementedJSON(w http.ResponseWriter) {
	WriteJSON(w, http.StatusNotImplemented, map[string]string{"detail": "not implemented yet"})
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func pathParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func preview(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

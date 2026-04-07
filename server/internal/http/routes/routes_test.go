package routes

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRegisterHealthRoutes(t *testing.T) {
	r := chi.NewRouter()

	RegisterHealthRoutes(r)

	assertRouteRegistered(t, r, http.MethodGet, "/health")
}

func TestRegisterOCRRoutes(t *testing.T) {
	r := chi.NewRouter()

	RegisterOCRRoutes(r)

	assertRouteRegistered(t, r, http.MethodPost, "/api/ocr/extract-text")
}

func assertRouteRegistered(t *testing.T, r chi.Router, method string, path string) {
	t.Helper()

	found := false
	if err := chi.Walk(r, func(routeMethod string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if routeMethod == method && route == path {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	if !found {
		t.Fatalf("expected %s %s to be registered", method, path)
	}
}

package http

import (
	"net/http"
	"testing"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func TestRegisterRoutesIncludesCoreEndpoints(t *testing.T) {
	r := chi.NewRouter()
	cfg := config.Config{
		AppPassword:          "test-password",
		AppSecretKey:         "test-secret",
		SessionMaxAgeSeconds: 3600,
		SecureCookies:        false,
	}
	sessionManager := middleware.NewSessionManager(cfg)

	registerRoutes(r, cfg, sessionManager)

	assertRouteRegistered(t, r, http.MethodGet, "/health")
	assertRouteRegistered(t, r, http.MethodPost, "/api/ocr/extract-text")
	assertRouteRegistered(t, r, http.MethodPost, "/api/auth/login")
	assertRouteRegistered(t, r, http.MethodGet, "/api/discovery/status")
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

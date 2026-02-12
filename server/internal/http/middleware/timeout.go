package middleware

import (
	"net/http"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TimeoutUnlessStream(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		withTimeout := chimiddleware.Timeout(timeout)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isTranslationStreamPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			withTimeout.ServeHTTP(w, r)
		})
	}
}

func isTranslationStreamPath(path string) bool {
	return strings.HasPrefix(path, "/api/translations/") && strings.HasSuffix(path, "/stream")
}

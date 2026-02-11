package handlers

import (
	"net/http"
	"os"
	"strings"
)

func StaticFileHandler(prefix string, dir string) http.Handler {
	fileServer := http.StripPrefix(prefix, http.FileServer(http.Dir(dir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(dir); err != nil {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "..") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

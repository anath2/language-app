package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/anath2/language-app/internal/config"
)

func SPAHTML(cfg config.Config) string {
	if cfg.ViteDevServer != "" {
		return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Language App</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="%s/src/main.ts"></script>
  </body>
</html>`, cfg.ViteDevServer)
	}

	indexPath := filepath.Join(cfg.WebDistDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err == nil {
		return string(content)
	}

	return "<!DOCTYPE html><html><body>App not built. Run `cd web && npm run build`</body></html>"
}

func ServeSPA(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(SPAHTML(cfg)))
	}
}

func SPAFallback(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, "css/") || strings.HasPrefix(path, "assets/") {
			http.NotFound(w, r)
			return
		}
		ServeSPA(cfg).ServeHTTP(w, r)
	}
}

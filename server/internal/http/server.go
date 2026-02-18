package http

import (
	"log"
	stdhttp "net/http"
	"time"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/anath2/language-app/internal/http/middleware"
	"github.com/anath2/language-app/internal/http/routes"
	ilchat "github.com/anath2/language-app/internal/intelligence/chat"
	iltrans "github.com/anath2/language-app/internal/intelligence/translation"
	"github.com/anath2/language-app/internal/migrations"
	"github.com/anath2/language-app/internal/queue"
	"github.com/anath2/language-app/internal/translation"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(cfg config.Config) stdhttp.Handler {
	if cfg.MigrationsDir != "" {
		if err := migrations.RunUp(cfg.TranslationDBPath, cfg.MigrationsDir); err != nil {
			log.Printf("failed to run migrations: %v", err)
			return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				w.WriteHeader(stdhttp.StatusInternalServerError)
				_, _ = w.Write([]byte("Server initialization error"))
			})
		}
	}

	db, err := translation.NewDB(cfg.TranslationDBPath)
	if err != nil {
		log.Printf("failed to initialize translation store: %v", err)
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			w.WriteHeader(stdhttp.StatusInternalServerError)
			_, _ = w.Write([]byte("Server initialization error"))
		})
	}
	translationStore := translation.NewTranslationStore(db)
	textEventStore := translation.NewTextEventStore(db)
	srsStore := translation.NewSRSStore(db)
	profileStore := translation.NewProfileStore(db)

	translationProv, err := iltrans.NewDSPyProvider(cfg)
	if err != nil {
		log.Printf("failed to initialize translation provider: %v", err)
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			w.WriteHeader(stdhttp.StatusInternalServerError)
			_, _ = w.Write([]byte("Server initialization error"))
		})
	}
	chatProv := ilchat.New(cfg)

	manager := queue.NewManager(translationStore, translationProv)
	handlers.ConfigureDependencies(translationStore, textEventStore, srsStore, profileStore, manager, translationProv, chatProv)
	manager.ResumeRestartableJobs()

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.TimeoutUnlessStream(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	sessionManager := middleware.NewSessionManager(cfg)
	r.Use(middleware.Auth(cfg, sessionManager))

	r.Get("/health", handlers.Health)
	r.Post("/api/extract-text", handlers.ExtractText)

	routes.RegisterAuthRoutes(r, cfg, sessionManager)
	routes.RegisterTranslationRoutes(r)
	routes.RegisterAPIRoutes(r)
	routes.RegisterAdminRoutes(r)

	return r
}

func ListenAndServe(addr string, cfg config.Config) error {
	return stdhttp.ListenAndServe(addr, NewRouter(cfg))
}

package http

import (
	"context"
	"fmt"
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
	"github.com/anath2/language-app/internal/storage"
	"github.com/anath2/language-app/internal/translation"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(cfg config.Config) stdhttp.Handler {
	if err := runMigrations(cfg); err != nil {
		return initializationErrorHandler(err)
	}

	if err := initDependencies(cfg); err != nil {
		return initializationErrorHandler(err)
	}

	r := chi.NewRouter()
	sessionManager := middleware.NewSessionManager(cfg)

	addMiddleware(r, cfg, sessionManager)
	registerRoutes(r, cfg, sessionManager)

	return r
}

func runMigrations(cfg config.Config) error {
	if cfg.MigrationsDir == "" {
		return nil
	}

	if err := migrations.RunUp(cfg.TranslationDBPath, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

func initDependencies(cfg config.Config) error {
	db, err := storage.NewDB(cfg.TranslationDBPath)
	if err != nil {
		return fmt.Errorf("initialize translation store: %w", err)
	}

	translationStore := translation.NewTranslationStore(db)
	chatStore := translation.NewChatStore(db)
	srsStore := translation.NewSRSStore(db)
	profileStore := translation.NewProfileStore(db)

	translationProv, err := iltrans.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("initialize translation provider: %w", err)
	}
	chatProv := ilchat.New(cfg)

	manager := queue.NewManager(translationStore, translationProv)
	handlers.ConfigureDependencies(translationStore, chatStore, srsStore, profileStore, manager, translationProv, chatProv)
	manager.ResumeRestartableJobs()
	manager.StartBackgroundScanner(context.Background())

	return nil
}

func addMiddleware(r chi.Router, cfg config.Config, sessionManager *middleware.SessionManager) {
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

	r.Use(middleware.Auth(cfg, sessionManager))
}

func registerRoutes(r chi.Router, cfg config.Config, sessionManager *middleware.SessionManager) {
	routes.RegisterHealthRoutes(r)
	routes.RegisterOCRRoutes(r)
	routes.RegisterAuthRoutes(r, cfg, sessionManager)
	routes.RegisterTranslationRoutes(r)
	routes.RegisterVocabRoutes(r)
	routes.RegisterReviewRoutes(r)
	routes.RegisterDiscoveryRoutes(r)
	routes.RegisterAdminRoutes(r)
}

func initializationErrorHandler(err error) stdhttp.Handler {
	log.Printf("server initialization failed: %v", err)

	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusInternalServerError)
		_, _ = w.Write([]byte("Server initialization error"))
	})
}

func ListenAndServe(addr string, cfg config.Config) error {
	return stdhttp.ListenAndServe(addr, NewRouter(cfg))
}

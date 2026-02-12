package main

import (
	"log"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/migrations"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := migrations.RunUp(cfg.TranslationDBPath, cfg.MigrationsDir); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	version, err := migrations.CurrentVersion(cfg.TranslationDBPath, cfg.MigrationsDir)
	if err != nil {
		log.Fatalf("failed to inspect migration version: %v", err)
	}
	log.Printf("migrations complete, current version=%d", version)
}

package main

import (
	"log"
	"os"

	"github.com/anath2/language-app/internal/config"
	httprouter "github.com/anath2/language-app/internal/http"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	addr := cfg.Addr
	if envPort := os.Getenv("PORT"); envPort != "" {
		addr = ":" + envPort
	}

	log.Printf("server listening on %s", addr)
	log.Fatal(httprouter.ListenAndServe(addr, cfg))
}

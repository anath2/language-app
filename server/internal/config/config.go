package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultSessionMaxAgeHours = 168

type Config struct {
	Addr                 string
	AppPassword          string
	AppSecretKey         string
	SessionMaxAgeSeconds int
	SecureCookies        bool
	ViteDevServer        string
	WebPublicCSSDir      string
	WebDistDir           string
	MigrationsDir        string
	TranslationDBPath    string
	CedictPath           string
	OpenRouterAPIKey     string
	OpenRouterModel      string
	OpenRouterBaseURL    string
	OpenRouterDebugLog   bool
}

func Load() (Config, error) {
	appPassword := os.Getenv("APP_PASSWORD")
	if appPassword == "" {
		return Config{}, fmt.Errorf("APP_PASSWORD environment variable is required")
	}

	appSecretKey := os.Getenv("APP_SECRET_KEY")
	if appSecretKey == "" {
		return Config{}, fmt.Errorf("APP_SECRET_KEY environment variable is required")
	}
	openRouterAPIKey := os.Getenv("OPENROUTER_API_KEY")
	if openRouterAPIKey == "" {
		return Config{}, fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
	}
	openRouterModel := os.Getenv("OPENROUTER_MODEL")
	if openRouterModel == "" {
		return Config{}, fmt.Errorf("OPENROUTER_MODEL environment variable is required")
	}

	sessionHours := defaultSessionMaxAgeHours
	if raw := os.Getenv("SESSION_MAX_AGE_HOURS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SESSION_MAX_AGE_HOURS: %w", err)
		}
		sessionHours = parsed
	}

	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	repoRoot, err := detectRepoRoot()
	if err != nil {
		return Config{}, err
	}

	secureCookies := strings.EqualFold(os.Getenv("SECURE_COOKIES"), "true")
	if os.Getenv("SECURE_COOKIES") == "" {
		secureCookies = true
	}

	return Config{
		Addr:                 addr,
		AppPassword:          appPassword,
		AppSecretKey:         appSecretKey,
		SessionMaxAgeSeconds: sessionHours * 3600,
		SecureCookies:        secureCookies,
		ViteDevServer:        os.Getenv("VITE_DEV_SERVER"),
		WebPublicCSSDir:      filepath.Join(repoRoot, "web", "public", "css"),
		WebDistDir:           filepath.Join(repoRoot, "web", "dist"),
		MigrationsDir:        envOrDefault("LANGUAGE_APP_MIGRATIONS_DIR", filepath.Join(repoRoot, "server", "migrations")),
		TranslationDBPath:    envOrDefault("LANGUAGE_APP_DB_PATH", filepath.Join(repoRoot, "server", "data", "language_app.db")),
		CedictPath:           envFirstOrDefault([]string{"CEDICT_PATH", "CEDIT_PATH", "CCEDICT_PATH"}, filepath.Join(repoRoot, "server", "data", "cedict_ts.u8")),
		OpenRouterAPIKey:     openRouterAPIKey,
		OpenRouterModel:      openRouterModel,
		OpenRouterBaseURL:    envOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		OpenRouterDebugLog:   strings.EqualFold(os.Getenv("OPENROUTER_DEBUG_LOG"), "true"),
	}, nil
}

func detectRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	if filepath.Base(wd) == "server" {
		return filepath.Dir(wd), nil
	}

	serverDir := filepath.Join(wd, "server")
	if _, err := os.Stat(serverDir); err == nil {
		return wd, nil
	}

	parent := filepath.Dir(wd)
	if filepath.Base(parent) == "server" {
		return filepath.Dir(parent), nil
	}

	return "", fmt.Errorf("could not detect repository root from working directory %q", wd)
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envFirstOrDefault(keys []string, fallback string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}

package config

import (
	"fmt"
	"net/url"
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
	OpenAIAPIKey         string
	OpenAIModel          string
	OpenAIBaseURL        string
	OpenAIDebugLog       bool
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
	openAIAPIKey := envFirstOrDefault([]string{"OPENAI_API_KEY", "OPENROUTER_API_KEY"}, "")
	if openAIAPIKey == "" {
		return Config{}, fmt.Errorf("OPENAI_API_KEY environment variable is required (or legacy OPENROUTER_API_KEY)")
	}
	openAIModel := envFirstOrDefault([]string{"OPENAI_MODEL", "OPENROUTER_MODEL"}, "")
	if openAIModel == "" {
		return Config{}, fmt.Errorf("OPENAI_MODEL environment variable is required (or legacy OPENROUTER_MODEL)")
	}
	openAIBaseURL, err := normalizeAndValidateOpenAIBaseURL(
		envFirstOrDefault([]string{"OPENAI_BASE_URL", "OPENROUTER_BASE_URL"}, "https://openrouter.ai/api/v1"),
	)
	if err != nil {
		return Config{}, fmt.Errorf("invalid OPENAI_BASE_URL: %w", err)
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
		OpenAIAPIKey:         openAIAPIKey,
		OpenAIModel:          openAIModel,
		OpenAIBaseURL:        openAIBaseURL,
		OpenAIDebugLog:       strings.EqualFold(envFirstOrDefault([]string{"OPENAI_DEBUG_LOG", "OPENROUTER_DEBUG_LOG"}, ""), "true"),
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

func normalizeAndValidateOpenAIBaseURL(raw string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return "", fmt.Errorf("must be a full URL ending with /v1")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("must include scheme and host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("must not include query string or fragment")
	}

	path := strings.TrimRight(parsed.Path, "/")
	if path == "" || !strings.HasSuffix(path, "/v1") {
		return "", fmt.Errorf("path must end with /v1")
	}
	if strings.Contains(path, "/chat/completions") {
		return "", fmt.Errorf("must be a base URL only; do not include /chat/completions")
	}

	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

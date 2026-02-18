package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestLoadRequiresOpenAIEnv(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	t.Setenv("APP_PASSWORD", "pw")
	t.Setenv("APP_SECRET_KEY", "secret")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENAI_TRANSLATION_MODEL", "")
	t.Setenv("OPENAI_CHAT_MODEL", "")
	t.Setenv("OPENROUTER_TRANSLATION_MODEL", "")
	t.Setenv("OPENROUTER_CHAT_MODEL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when OPENAI vars are missing")
	}
}

func TestLoadFromDotenvThenValidate(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	envPath := filepath.Join(repoRoot, ".env")
	envContent := "APP_PASSWORD=testpass\nAPP_SECRET_KEY=testsecret\nOPENAI_API_KEY=oa-key\nOPENAI_TRANSLATION_MODEL=openai/gpt-4o-mini\nOPENAI_CHAT_MODEL=openai/gpt-4o-mini\nSECURE_COOKIES=false\nCEDICT_PATH=custom/cedict_ts.u8\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("APP_PASSWORD", "")
	t.Setenv("APP_SECRET_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENAI_TRANSLATION_MODEL", "")
	t.Setenv("OPENAI_CHAT_MODEL", "")
	t.Setenv("OPENROUTER_TRANSLATION_MODEL", "")
	t.Setenv("OPENROUTER_CHAT_MODEL", "")
	t.Setenv("SECURE_COOKIES", "")

	if err := godotenv.Overload(envPath); err != nil {
		t.Fatalf("load dotenv: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.OpenAIAPIKey != "oa-key" {
		t.Fatalf("unexpected OPENAI_API_KEY: %q", cfg.OpenAIAPIKey)
	}
	if cfg.OpenAITranslationModel != "openai/gpt-4o-mini" {
		t.Fatalf("unexpected OPENAI_TRANSLATION_MODEL: %q", cfg.OpenAITranslationModel)
	}
	if cfg.SecureCookies {
		t.Fatal("expected secure cookies false from dotenv")
	}
	if cfg.CedictPath != "custom/cedict_ts.u8" {
		t.Fatalf("unexpected CEDICT_PATH: %q", cfg.CedictPath)
	}
}

func TestLoadCedictPathAliases(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	t.Setenv("APP_PASSWORD", "pw")
	t.Setenv("APP_SECRET_KEY", "secret")
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("OPENAI_TRANSLATION_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENAI_CHAT_MODEL", "openai/gpt-4o-mini")
	t.Setenv("CEDICT_PATH", "")
	t.Setenv("CEDIT_PATH", "alias/cedit_path.u8")
	t.Setenv("CCEDICT_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CedictPath != "alias/cedit_path.u8" {
		t.Fatalf("unexpected cedit alias path: %q", cfg.CedictPath)
	}
}

func TestLoadValidatesOpenAIBaseURL(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	t.Setenv("APP_PASSWORD", "pw")
	t.Setenv("APP_SECRET_KEY", "secret")
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("OPENAI_TRANSLATION_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENAI_CHAT_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENAI_BASE_URL", "https://openrouter.ai/api/v1/chat/completions")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid OPENAI_BASE_URL")
	}
}

func TestLoadNormalizesOpenAIBaseURL(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	t.Setenv("APP_PASSWORD", "pw")
	t.Setenv("APP_SECRET_KEY", "secret")
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("OPENAI_TRANSLATION_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENAI_CHAT_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENAI_BASE_URL", "http://127.0.0.1:11434/v1/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.OpenAIBaseURL != "http://127.0.0.1:11434/v1" {
		t.Fatalf("unexpected normalized OPENAI_BASE_URL: %q", cfg.OpenAIBaseURL)
	}
}

func TestLoadSupportsLegacyOpenRouterEnvNames(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	t.Setenv("APP_PASSWORD", "pw")
	t.Setenv("APP_SECRET_KEY", "secret")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_TRANSLATION_MODEL", "")
	t.Setenv("OPENAI_CHAT_MODEL", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("OPENROUTER_TRANSLATION_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENROUTER_CHAT_MODEL", "openai/gpt-4o-mini")
	t.Setenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.OpenAIAPIKey != "or-key" {
		t.Fatalf("unexpected API key from legacy env: %q", cfg.OpenAIAPIKey)
	}
	if cfg.OpenAITranslationModel != "openai/gpt-4o-mini" {
		t.Fatalf("unexpected translation model from legacy env: %q", cfg.OpenAITranslationModel)
	}
	if cfg.OpenAIBaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("unexpected base URL from legacy env: %q", cfg.OpenAIBaseURL)
	}
}

func createTempRepoRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "server"), 0o755); err != nil {
		t.Fatalf("create server dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "web", "public", "css"), 0o755); err != nil {
		t.Fatalf("create web css dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "web", "dist"), 0o755); err != nil {
		t.Fatalf("create web dist dir: %v", err)
	}
	return root
}

func withChdir(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

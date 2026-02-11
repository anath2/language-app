package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestLoadRequiresOpenRouterEnv(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	t.Setenv("APP_PASSWORD", "pw")
	t.Setenv("APP_SECRET_KEY", "secret")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_MODEL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when OPENROUTER vars are missing")
	}
}

func TestLoadFromDotenvThenValidate(t *testing.T) {
	repoRoot := createTempRepoRoot(t)
	withChdir(t, repoRoot)

	envPath := filepath.Join(repoRoot, ".env")
	envContent := "APP_PASSWORD=testpass\nAPP_SECRET_KEY=testsecret\nOPENROUTER_API_KEY=or-key\nOPENROUTER_MODEL=openai/gpt-4o-mini\nSECURE_COOKIES=false\nCEDICT_PATH=custom/cedict_ts.u8\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("APP_PASSWORD", "")
	t.Setenv("APP_SECRET_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_MODEL", "")
	t.Setenv("SECURE_COOKIES", "")

	if err := godotenv.Overload(envPath); err != nil {
		t.Fatalf("load dotenv: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.OpenRouterAPIKey != "or-key" {
		t.Fatalf("unexpected OPENROUTER_API_KEY: %q", cfg.OpenRouterAPIKey)
	}
	if cfg.OpenRouterModel != "openai/gpt-4o-mini" {
		t.Fatalf("unexpected OPENROUTER_MODEL: %q", cfg.OpenRouterModel)
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
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("OPENROUTER_MODEL", "openai/gpt-4o-mini")
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

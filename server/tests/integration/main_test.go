package integration_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

var upstream = flag.Bool("upstream", false, "run integration tests that hit upstream LLM APIs")

func TestMain(m *testing.M) {
	flag.Parse()

	if *upstream {
		if err := loadUpstreamEnv(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to load upstream test env: %v\n", err)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

func requireUpstream(t *testing.T) {
	t.Helper()

	if !*upstream {
		t.Skip("skipping upstream integration test; pass -upstream to enable")
	}
}

func loadUpstreamEnv() error {
	serverRoot, err := detectServerRootFromWD()
	if err != nil {
		return err
	}

	envFile := os.Getenv("ENV_FILE")
	if strings.TrimSpace(envFile) == "" {
		envFile = ".env.test"
	}
	if !filepath.IsAbs(envFile) {
		envFile = filepath.Join(serverRoot, envFile)
	}

	if err := godotenv.Overload(envFile); err != nil {
		return fmt.Errorf("load env file %q: %w", envFile, err)
	}

	return nil
}

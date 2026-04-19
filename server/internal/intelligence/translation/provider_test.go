package translation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/anath2/language-app/internal/config"
)

func mockCompletionServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
		})
	}))
}

func newTestProvider(t *testing.T, srv *httptest.Server) *Provider {
	t.Helper()
	return &Provider{
		client:      srv.Client(),
		baseURL:     srv.URL,
		apiKey:      "test-key",
		model:       "test-model",
		instruction: "test instruction",
	}
}

func TestProvider_Segment(t *testing.T) {
	t.Parallel()
	srv := mockCompletionServer(t, `{"segments":["你好","世界"]}`)
	defer srv.Close()

	p := newTestProvider(t, srv)
	segments, err := p.Segment(context.Background(), "你好世界")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segments) != 2 || segments[0] != "你好" || segments[1] != "世界" {
		t.Fatalf("unexpected segments: %v", segments)
	}
}

func TestProvider_Segment_EmptyInput(t *testing.T) {
	t.Parallel()
	p := &Provider{client: &http.Client{}, baseURL: "http://unused", model: "m", instruction: "i"}
	segments, err := p.Segment(context.Background(), "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segments) != 0 {
		t.Fatalf("expected empty, got %v", segments)
	}
}

func TestProvider_Segment_UpstreamError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	_, err := p.Segment(context.Background(), "你好")
	if err == nil {
		t.Fatal("expected error for upstream 500, got nil")
	}
}

func TestProvider_Segment_NoChoices(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	_, err := p.Segment(context.Background(), "你好")
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestProvider_TranslateSentenceSegments_SkipsNonCJK(t *testing.T) {
	t.Parallel()
	p := &Provider{client: &http.Client{}, baseURL: "http://unused", model: "m", instruction: "i"}
	results, err := p.TranslateSentenceSegments(context.Background(), []string{"。", "!", " "}, "test", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Pinyin != "" || r.English != "" {
			t.Fatalf("result[%d] should have empty fields, got pinyin=%q english=%q", i, r.Pinyin, r.English)
		}
	}
}

func TestProvider_TranslateSentenceSegments_HappyPath(t *testing.T) {
	t.Parallel()
	srv := mockCompletionServer(t, `{"translations":[{"pinyin":"nǐ hǎo","english":"hello"},{"pinyin":"shì jiè","english":"world"}]}`)
	defer srv.Close()

	p := newTestProvider(t, srv)
	results, err := p.TranslateSentenceSegments(context.Background(), []string{"你好", "世界"}, "你好世界", "你好世界")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 || results[0].Pinyin != "nǐ hǎo" || results[0].English != "hello" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestProvider_TranslateFull(t *testing.T) {
	t.Parallel()
	srv := mockCompletionServer(t, `{"translation":"Hello, world!"}`)
	defer srv.Close()

	p := newTestProvider(t, srv)
	got, err := p.TranslateFull(context.Background(), "你好世界")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello, world!" {
		t.Fatalf("unexpected translation: %q", got)
	}
}

func TestProvider_RequestHasBearerToken(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": `{"segments":["测试"]}`}}},
		})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	p.apiKey = "sk-secret"
	_, _ = p.Segment(context.Background(), "测试")
	if gotAuth != "Bearer sk-secret" {
		t.Fatalf("expected 'Bearer sk-secret', got %q", gotAuth)
	}
}

func TestNormalizeOpenAIEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input   string
		wantErr bool
	}{
		{"https://openrouter.ai/api/v1", false},
		{"https://api.openai.com/v1", false},
		{"https://api.openai.com/v1/", false},
		{"", true},
		{"https://api.openai.com", true},
		{"https://api.openai.com/v1/chat/completions", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			_, _, err := normalizeOpenAIEndpoint(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
		})
	}
}

func TestLoadCompiledSegmentationInstruction_PrefersRepoRootPath(t *testing.T) {
	tempDir := t.TempDir()
	rootPath := filepath.Join(tempDir, "data", "jepa", "compiled_instruction.txt")
	legacyPath := filepath.Join(tempDir, "server", "data", "jepa", "compiled_instruction.txt")

	if err := os.MkdirAll(filepath.Dir(rootPath), 0o755); err != nil {
		t.Fatalf("mkdir root path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy path: %v", err)
	}
	if err := os.WriteFile(rootPath, []byte("repo root instruction\n"), 0o644); err != nil {
		t.Fatalf("write root instruction: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy instruction\n"), 0o644); err != nil {
		t.Fatalf("write legacy instruction: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	got := loadCompiledSegmentationInstruction(config.Config{})
	if got != "repo root instruction" {
		t.Fatalf("expected repo root instruction, got %q", got)
	}
}

func TestLoadCompiledSegmentationInstruction_FallsBackToLegacyServerPath(t *testing.T) {
	tempDir := t.TempDir()
	legacyPath := filepath.Join(tempDir, "server", "data", "jepa", "compiled_instruction.txt")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy path: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy instruction\n"), 0o644); err != nil {
		t.Fatalf("write legacy instruction: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	got := loadCompiledSegmentationInstruction(config.Config{})
	if got != "legacy instruction" {
		t.Fatalf("expected legacy fallback instruction, got %q", got)
	}
}

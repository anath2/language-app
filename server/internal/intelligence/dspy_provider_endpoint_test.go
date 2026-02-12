package intelligence

import "testing"

func TestNormalizeOpenAIEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantBaseURL string
		wantPath    string
		wantErr     bool
	}{
		{
			name:        "valid openrouter api v1",
			input:       "https://openrouter.ai/api/v1",
			wantBaseURL: "https://openrouter.ai/api/v1",
			wantPath:    "/chat/completions",
		},
		{
			name:        "valid custom host with trailing slash",
			input:       "http://127.0.0.1:11434/v1/",
			wantBaseURL: "http://127.0.0.1:11434/v1",
			wantPath:    "/chat/completions",
		},
		{
			name:    "reject empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "reject path without v1 suffix",
			input:   "https://openrouter.ai/api",
			wantErr: true,
		},
		{
			name:    "reject full endpoint path",
			input:   "https://openrouter.ai/api/v1/chat/completions",
			wantErr: true,
		},
		{
			name:    "reject malformed URL",
			input:   "://openrouter.ai/api/v1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotBaseURL, gotPath, err := normalizeOpenAIEndpoint(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (baseURL=%q path=%q)", gotBaseURL, gotPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotBaseURL != tc.wantBaseURL {
				t.Fatalf("baseURL mismatch: got %q want %q", gotBaseURL, tc.wantBaseURL)
			}
			if gotPath != tc.wantPath {
				t.Fatalf("path mismatch: got %q want %q", gotPath, tc.wantPath)
			}
		})
	}
}

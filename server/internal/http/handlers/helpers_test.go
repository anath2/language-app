package handlers

import "testing"

func TestPreview(t *testing.T) {
	tests := []struct {
		name string
		text string
		max  int
		want string
	}{
		{
			name: "short ascii text unchanged",
			text: "hello",
			max:  10,
			want: "hello",
		},
		{
			name: "long ascii text truncated",
			text: "hello world",
			max:  5,
			want: "hello...",
		},
		{
			name: "unicode text truncated on rune boundary",
			text: "你好世界和平",
			max:  3,
			want: "你好世...",
		},
		{
			name: "max zero returns only ellipsis for non-empty",
			text: "abc",
			max:  0,
			want: "...",
		},
		{
			name: "negative max behaves like zero",
			text: "abc",
			max:  -1,
			want: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preview(tt.text, tt.max)
			if got != tt.want {
				t.Fatalf("preview(%q, %d) = %q, want %q", tt.text, tt.max, got, tt.want)
			}
		})
	}
}

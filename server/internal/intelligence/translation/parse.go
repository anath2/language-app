package translation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// segmentsKeyPrefix matches "segments:" or "Segments: " etc. (field-name prefix from structured output)
var segmentsKeyPrefix = regexp.MustCompile(`(?i)^\s*segments\s*:\s*`)

func isMetadataSegment(s string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(s))
	// Filter only pure metadata (field-name leakage), not content that contains "segments"
	return trimmed == "segments" || trimmed == "segments:" || trimmed == "segments: "
}

func parseSegments(v any) []string {
	if v == nil {
		return nil
	}
	switch items := v.(type) {
	case []any:
		out := make([]string, 0, len(items))
		for _, it := range items {
			s := strings.TrimSpace(toString(it))
			if s != "" && !isMetadataSegment(s) {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(items))
		for _, it := range items {
			s := strings.TrimSpace(it)
			if s != "" && !isMetadataSegment(s) {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(toString(v))
		if s == "" {
			return nil
		}
		return parseSegmentsString(s)
	}
}

func parseSegmentsFromResponse(v any) []string {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		segments := parseSegments(m["segments"])
		if len(segments) > 0 {
			return segments
		}
	}
	raw := normalizeJSONLikePayload(strings.TrimSpace(toString(v)))
	if raw == "" {
		return nil
	}
	if segments := parseSegmentsString(raw); len(segments) > 0 {
		return segments
	}
	// Handle newline-separated format: "segments:\n如何\n评价\n..."
	if segments := parseNewlineSegments(raw); len(segments) > 0 {
		return segments
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return parseSegments(payload["segments"])
}

// extractJSONArray finds the first [...] in s and unmarshals it. Handles freeform text like "segments: [...]".
func extractJSONArray(s string) []any {
	start := strings.Index(s, "[")
	if start < 0 {
		return nil
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				var out []any
				if err := json.Unmarshal([]byte(s[start:i+1]), &out); err == nil {
					return out
				}
				return nil
			}
		}
	}
	return nil
}

func parseSegmentsString(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// Strip "segments:" or "Segments : " prefix (from structured output field name)
	raw = strings.TrimSpace(segmentsKeyPrefix.ReplaceAllString(raw, ""))
	if raw == "" {
		return nil
	}

	// Try direct JSON parse
	var listPayload []any
	if err := json.Unmarshal([]byte(raw), &listPayload); err == nil {
		return parseSegments(listPayload)
	}

	// Try JSON object with "segments" key
	var mapPayload map[string]any
	if err := json.Unmarshal([]byte(raw), &mapPayload); err == nil {
		if segs := parseSegments(mapPayload["segments"]); len(segs) > 0 {
			return segs
		}
	}

	// Extract JSON array from freeform text (e.g. "Here are the segments: [...]")
	if arr := extractJSONArray(raw); len(arr) > 0 {
		return parseSegments(arr)
	}
	return nil
}

// parseNewlineSegments handles "segments:\n如何\n评价\n..." format (one segment per line).
func parseNewlineSegments(raw string) []string {
	raw = strings.TrimSpace(segmentsKeyPrefix.ReplaceAllString(raw, ""))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s != "" && !isMetadataSegment(s) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLooseSegments splits on whitespace, commas, and pipes as a last-resort fallback
// when the model returns non-JSON output (e.g. space-separated words).
func parseLooseSegments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, ",", " ")
	raw = strings.ReplaceAll(raw, "|", " ")
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func parseTranslationFromResponse(v any) (string, string) {
	if v == nil {
		return "", ""
	}
	if m, ok := v.(map[string]any); ok {
		return normalizeModelField(toString(m["pinyin"])), normalizeModelField(toString(m["english"]))
	}
	raw := normalizeJSONLikePayload(strings.TrimSpace(toString(v)))
	if raw == "" {
		return "", ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	return normalizeModelField(toString(payload["pinyin"])), normalizeModelField(toString(payload["english"]))
}

func parseFullTranslationFromResponse(v any) string {
	if v == nil {
		return ""
	}
	if m, ok := v.(map[string]any); ok {
		if t := strings.TrimSpace(toString(m["translation"])); t != "" {
			return t
		}
	}
	raw := normalizeJSONLikePayload(strings.TrimSpace(toString(v)))
	if raw == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		if t := strings.TrimSpace(toString(payload["translation"])); t != "" {
			return t
		}
	}
	// If the response is not JSON, treat the raw string as the translation itself.
	return strings.TrimSpace(raw)
}

func normalizeJSONLikePayload(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Handle markdown fenced payloads like:
	// ```json
	// {"pinyin":"...","english":"..."}
	// ```
	if strings.HasPrefix(raw, "```") {
		parts := strings.Split(raw, "\n")
		if len(parts) >= 2 {
			parts = parts[1:]
		}
		if len(parts) > 0 {
			last := strings.TrimSpace(parts[len(parts)-1])
			if strings.HasPrefix(last, "```") {
				parts = parts[:len(parts)-1]
			}
		}
		raw = strings.TrimSpace(strings.Join(parts, "\n"))
	}

	// Some providers prepend "json" without fences.
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "json"))
	return raw
}

func normalizeModelField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "pinyin:") {
		value = strings.TrimSpace(value[len("pinyin:"):])
	}
	lower = strings.ToLower(value)
	if strings.HasPrefix(lower, "english:") {
		value = strings.TrimSpace(value[len("english:"):])
	}
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") && len(value) > 2 {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

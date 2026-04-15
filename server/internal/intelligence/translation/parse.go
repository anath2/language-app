package translation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type batchTranslation struct {
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

// parseSegmentsResult unmarshals {"segments": [...]} from a json_schema response.
func parseSegmentsResult(content string) ([]string, error) {
	var result struct {
		Segments []string `json:"segments"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse segments JSON: %w", err)
	}
	if len(result.Segments) == 0 {
		return nil, fmt.Errorf("parse segments: empty result")
	}
	return result.Segments, nil
}

// parseBatchTranslationsResult unmarshals {"translations": [{pinyin, english}, ...]} from a json_schema response.
func parseBatchTranslationsResult(content string) ([]batchTranslation, error) {
	var result struct {
		Translations []batchTranslation `json:"translations"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse batch translations JSON: %w", err)
	}
	return result.Translations, nil
}

// parseFullTranslationResult unmarshals {"translation": "..."} from a json_schema response.
func parseFullTranslationResult(content string) (string, error) {
	var result struct {
		Translation string `json:"translation"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", fmt.Errorf("parse full translation JSON: %w", err)
	}
	if strings.TrimSpace(result.Translation) == "" {
		return "", fmt.Errorf("parse full translation: empty result")
	}
	return strings.TrimSpace(result.Translation), nil
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

func preview(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

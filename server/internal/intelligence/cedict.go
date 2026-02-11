package intelligence

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

var cedictEntryPattern = regexp.MustCompile(`^(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+/(.+)/$`)

type cedictEntry struct {
	Pinyin     string
	Definition string
}

type cedictDictionary struct {
	entries map[string]cedictEntry
}

func loadCedictDictionary(path string) (*cedictDictionary, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cedict: %w", err)
	}
	defer file.Close()

	dict := &cedictDictionary{entries: make(map[string]cedictEntry)}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "%") {
			continue
		}
		match := cedictEntryPattern.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}

		simplified := match[2]
		pinyinNumbered := strings.TrimSpace(match[3])
		defs := splitDefinitions(match[4])
		definition := strings.Join(defs, " / ")
		if definition == "" {
			continue
		}

		// Preserve the first entry for stability, mirroring deterministic lookup behavior.
		if _, exists := dict.entries[simplified]; exists {
			continue
		}
		dict.entries[simplified] = cedictEntry{
			Pinyin:     numberedPinyinToToneMarks(pinyinNumbered),
			Definition: definition,
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan cedict: %w", err)
	}
	return dict, nil
}

func (c *cedictDictionary) Lookup(word string) (cedictEntry, bool) {
	if c == nil {
		return cedictEntry{}, false
	}
	entry, ok := c.entries[word]
	return entry, ok
}

func splitDefinitions(raw string) []string {
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func numberedPinyinToToneMarks(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, convertPinyinSyllable(field))
	}
	return strings.Join(out, " ")
}

func convertPinyinSyllable(s string) string {
	if s == "" {
		return s
	}

	tone := 5
	lastRune, size := utf8.DecodeLastRuneInString(s)
	base := s
	if lastRune >= '0' && lastRune <= '9' {
		tone = int(lastRune - '0')
		base = s[:len(s)-size]
	}
	base = strings.ReplaceAll(base, "u:", "ü")
	base = strings.ReplaceAll(base, "U:", "Ü")
	base = strings.ReplaceAll(base, "v", "ü")
	base = strings.ReplaceAll(base, "V", "Ü")

	if tone <= 0 || tone == 5 {
		return base
	}

	runes := []rune(base)
	idx := toneVowelIndex(runes)
	if idx < 0 {
		return base
	}

	replacement, ok := toneMarkedVowel(runes[idx], tone)
	if !ok {
		return base
	}
	runes[idx] = replacement
	return string(runes)
}

func toneVowelIndex(runes []rune) int {
	for i, r := range runes {
		if r == 'a' || r == 'A' {
			return i
		}
	}
	for i, r := range runes {
		if r == 'e' || r == 'E' {
			return i
		}
	}
	for i := 0; i < len(runes)-1; i++ {
		if (runes[i] == 'o' || runes[i] == 'O') && (runes[i+1] == 'u' || runes[i+1] == 'U') {
			return i
		}
	}
	for i := len(runes) - 1; i >= 0; i-- {
		if isPinyinVowel(runes[i]) {
			return i
		}
	}
	return -1
}

func isPinyinVowel(r rune) bool {
	switch r {
	case 'a', 'A', 'e', 'E', 'i', 'I', 'o', 'O', 'u', 'U', 'ü', 'Ü':
		return true
	default:
		return false
	}
}

func toneMarkedVowel(r rune, tone int) (rune, bool) {
	switch r {
	case 'a':
		return []rune{'a', 'ā', 'á', 'ǎ', 'à'}[tone], true
	case 'A':
		return []rune{'A', 'Ā', 'Á', 'Ǎ', 'À'}[tone], true
	case 'e':
		return []rune{'e', 'ē', 'é', 'ě', 'è'}[tone], true
	case 'E':
		return []rune{'E', 'Ē', 'É', 'Ě', 'È'}[tone], true
	case 'i':
		return []rune{'i', 'ī', 'í', 'ǐ', 'ì'}[tone], true
	case 'I':
		return []rune{'I', 'Ī', 'Í', 'Ǐ', 'Ì'}[tone], true
	case 'o':
		return []rune{'o', 'ō', 'ó', 'ǒ', 'ò'}[tone], true
	case 'O':
		return []rune{'O', 'Ō', 'Ó', 'Ǒ', 'Ò'}[tone], true
	case 'u':
		return []rune{'u', 'ū', 'ú', 'ǔ', 'ù'}[tone], true
	case 'U':
		return []rune{'U', 'Ū', 'Ú', 'Ǔ', 'Ù'}[tone], true
	case 'ü':
		return []rune{'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ'}[tone], true
	case 'Ü':
		return []rune{'Ü', 'Ǖ', 'Ǘ', 'Ǚ', 'Ǜ'}[tone], true
	default:
		return r, false
	}
}

package translation

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
	Pinyin         string
	PinyinNumbered string
	Definition     string
}

type cedictDictionary struct {
	entries map[string][]cedictEntry
}

func loadCedictDictionary(path string) (*cedictDictionary, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cedict: %w", err)
	}
	defer file.Close()

	dict := &cedictDictionary{entries: make(map[string][]cedictEntry)}
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

		dict.entries[simplified] = append(dict.entries[simplified], cedictEntry{
			Pinyin:         numberedPinyinToToneMarks(pinyinNumbered),
			PinyinNumbered: pinyinNumbered,
			Definition:     definition,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan cedict: %w", err)
	}
	return dict, nil
}

// Lookup returns all entries for a word.
func (c *cedictDictionary) Lookup(word string) ([]cedictEntry, bool) {
	if c == nil {
		return nil, false
	}
	entries, ok := c.entries[word]
	return entries, ok
}

// LookupFirst returns the first entry for a word (backward compat convenience).
func (c *cedictDictionary) LookupFirst(word string) (cedictEntry, bool) {
	entries, ok := c.Lookup(word)
	if !ok || len(entries) == 0 {
		return cedictEntry{}, false
	}
	return entries[0], true
}

// IsCharAmbiguous returns true if a single character has multiple entries with
// genuinely different pinyin bases. Characters with a tone-5 (neutral/particle)
// reading (like 吗, 呢) are treated as unambiguous.
func (c *cedictDictionary) IsCharAmbiguous(char rune) bool {
	entries, ok := c.Lookup(string(char))
	if !ok || len(entries) <= 1 {
		return false
	}
	return hasDistinctPinyin(entries)
}

// PreferredCharPinyin returns the preferred pinyin for a single character.
// For particles (entries with a tone-5 reading), the tone-5 reading is preferred.
// Otherwise returns the first entry's pinyin.
func (c *cedictDictionary) PreferredCharPinyin(char rune) (string, bool) {
	entries, ok := c.Lookup(string(char))
	if !ok || len(entries) == 0 {
		return "", false
	}
	// Prefer tone-5 (neutral) reading for particles.
	for _, e := range entries {
		if isTone5Entry(e) {
			return e.Pinyin, true
		}
	}
	return entries[0].Pinyin, true
}

// ComposeSegmentPinyin tries to resolve pinyin for a segment without LLM.
// Returns (pinyin, true) if fully resolved, ("", false) if LLM is needed.
func (c *cedictDictionary) ComposeSegmentPinyin(segment string) (string, bool) {
	if c == nil {
		return "", false
	}

	// Try word-level lookup first.
	entries, ok := c.Lookup(segment)
	if ok && len(entries) > 0 {
		if !hasDistinctPinyin(entries) {
			return entries[0].Pinyin, true
		}
		// Multiple distinct pinyin readings at word level — need LLM.
		return "", false
	}

	// Fall through to character-level composition.
	var parts []string
	for _, r := range segment {
		if !isCJKIdeograph(r) {
			continue
		}
		if c.IsCharAmbiguous(r) {
			return "", false
		}
		py, found := c.PreferredCharPinyin(r)
		if !found {
			return "", false
		}
		parts = append(parts, py)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " "), true
}

// hasDistinctPinyin returns true if entries have more than one distinct pinyin
// base syllable (ignoring tone numbers). Tone-5 (neutral) entries are excluded
// from the comparison so particles don't trigger false ambiguity.
func hasDistinctPinyin(entries []cedictEntry) bool {
	seen := make(map[string]struct{})
	for _, e := range entries {
		if isTone5Entry(e) {
			continue
		}
		base := normalizePinyinBase(e.PinyinNumbered)
		seen[base] = struct{}{}
	}
	return len(seen) > 1
}

// isTone5Entry returns true if all syllables in the entry's numbered pinyin
// end with tone 5 (neutral tone) or have no tone number.
func isTone5Entry(e cedictEntry) bool {
	fields := strings.Fields(e.PinyinNumbered)
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		last, _ := utf8.DecodeLastRuneInString(f)
		if last == '5' || (last < '0' || last > '9') {
			continue
		}
		return false
	}
	return true
}

// normalizePinyinBase strips tone numbers from numbered pinyin and lowercases.
func normalizePinyinBase(numbered string) string {
	fields := strings.Fields(strings.ToLower(numbered))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		last, size := utf8.DecodeLastRuneInString(f)
		if last >= '0' && last <= '9' {
			f = f[:len(f)-size]
		}
		// Normalize ü variants.
		f = strings.ReplaceAll(f, "u:", "v")
		f = strings.ReplaceAll(f, "ü", "v")
		out = append(out, f)
	}
	return strings.Join(out, " ")
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

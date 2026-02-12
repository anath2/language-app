package intelligence

import (
	"strings"
	"unicode"
)

const chinesePunctuation = "，。！？；：、（）【】《》〈〉「」『』“”‘’—…·"

func isCJKIdeograph(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // Main CJK block
		(r >= 0x3400 && r <= 0x4DBF) || // Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // Extension B
		(r >= 0x2A700 && r <= 0x2CEAF) || // Extensions C-E
		(r >= 0x2CEB0 && r <= 0x2EBEF) || // Extensions F-I
		(r >= 0x30000 && r <= 0x323AF) // Extensions G-H
}

func shouldSkipSegment(segment string) bool {
	if strings.TrimSpace(segment) == "" {
		return true
	}

	hasCJK := false
	for _, r := range segment {
		if isCJKIdeograph(r) {
			hasCJK = true
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		if r <= unicode.MaxASCII && !unicode.IsLetter(r) {
			continue
		}
		if strings.ContainsRune(chinesePunctuation, r) {
			continue
		}
		if unicode.In(r, unicode.Nd, unicode.No, unicode.Po, unicode.Ps, unicode.Pe, unicode.Pd, unicode.Pc, unicode.Sk, unicode.Sm, unicode.So) {
			continue
		}
	}
	return !hasCJK
}

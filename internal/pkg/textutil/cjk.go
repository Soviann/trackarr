package textutil

import (
	"fmt"
	"strings"
	"unicode"
)

// IsCJK checks if a string consists primarily of CJK characters (Han, Hiragana, Katakana).
func IsCJK(s string) bool {
	cjkCount := 0
	totalCount := 0
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsDigit(r) || unicode.IsSymbol(r) {
			continue
		}
		totalCount++
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			cjkCount++
		}
	}
	if totalCount == 0 {
		return false
	}
	return float64(cjkCount)/float64(totalCount) > 0.3
}

// ContainsCJK checks if a string contains any CJK characters.
func ContainsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

// SanitizeEpisodeName sanitizes an episode name, ensuring CJK or empty names fallback to English or "Épisode X".
func SanitizeEpisodeName(episodeNum int, currentName, fallbackEnglish string) string {
	trimmedCurrent := strings.TrimSpace(currentName)
	if trimmedCurrent != "" && !IsCJK(trimmedCurrent) {
		return trimmedCurrent
	}
	trimmedFallback := strings.TrimSpace(fallbackEnglish)
	if trimmedFallback != "" && !IsCJK(trimmedFallback) {
		return trimmedFallback
	}
	return fmt.Sprintf("Épisode %d", episodeNum)
}

// SelectBestTitle returns the best title among Romaji, English, and Native, filtering CJK titles.
func SelectBestTitle(romaji, english, native string) string {
	eng := strings.TrimSpace(english)
	if eng != "" && !IsCJK(eng) {
		return eng
	}
	rom := strings.TrimSpace(romaji)
	if rom != "" && !IsCJK(rom) {
		return rom
	}
	nat := strings.TrimSpace(native)
	if nat != "" && !IsCJK(nat) {
		return nat
	}
	return ""
}

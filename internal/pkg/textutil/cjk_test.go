package textutil_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/pkg/textutil"
)

func TestIsCJK(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Japanese Kanji/Kana", "キャプテン翼", true},
		{"Episode with Kanji", "第1話 挑戦", true},
		{"English title", "Captain Tsubasa", false},
		{"French episode title", "Épisode 1 - Grand départ", false},
		{"Empty string", "", false},
		{"Only numbers/punctuation", "123 - !!!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := textutil.IsCJK(tt.input)
			if res != tt.expected {
				t.Errorf("IsCJK(%q) = %v, want %v", tt.input, res, tt.expected)
			}
		})
	}
}

func TestContainsCJK(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"キャプテン翼", true},
		{"Captain Tsubasa (キャプテン翼)", true},
		{"Captain Tsubasa", false},
		{"Épisode 1", false},
	}

	for _, tt := range tests {
		if res := textutil.ContainsCJK(tt.input); res != tt.expected {
			t.Errorf("ContainsCJK(%q) = %v, want %v", tt.input, res, tt.expected)
		}
	}
}

func TestSanitizeEpisodeName(t *testing.T) {
	tests := []struct {
		epNum    int
		current  string
		fallback string
		expected string
	}{
		{1, "Le Grand Départ", "The Great Start", "Le Grand Départ"},
		{1, "第1話", "The Great Start", "The Great Start"},
		{1, "第1話", "第1話", "Épisode 1"},
		{2, "", "", "Épisode 2"},
		{3, "", "The Challenge", "The Challenge"},
	}

	for _, tt := range tests {
		res := textutil.SanitizeEpisodeName(tt.epNum, tt.current, tt.fallback)
		if res != tt.expected {
			t.Errorf("SanitizeEpisodeName(%d, %q, %q) = %q, want %q", tt.epNum, tt.current, tt.fallback, res, tt.expected)
		}
	}
}

func TestSelectBestTitle(t *testing.T) {
	tests := []struct {
		romaji   string
		english  string
		native   string
		expected string
	}{
		{"Kyaptin Tsubasa", "Captain Tsubasa", "キャプテン翼", "Captain Tsubasa"},
		{"Kyaptin Tsubasa", "", "キャプテン翼", "Kyaptin Tsubasa"},
		{"", "", "キャプテン翼", ""},
		{"", "Captain Tsubasa", "", "Captain Tsubasa"},
	}

	for _, tt := range tests {
		res := textutil.SelectBestTitle(tt.romaji, tt.english, tt.native)
		if res != tt.expected {
			t.Errorf("SelectBestTitle(%q, %q, %q) = %q, want %q", tt.romaji, tt.english, tt.native, res, tt.expected)
		}
	}
}

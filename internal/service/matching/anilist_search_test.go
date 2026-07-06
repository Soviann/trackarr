package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCountry(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want *string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"lowercase gets uppercased", "jp", strPtr("JP")},
		{"trims surrounding whitespace", "  KR  ", strPtr("KR")},
		{"already uppercase", "CN", strPtr("CN")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCountry(tt.in)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func strPtr(s string) *string { return &s }

func TestCleanAniListDescription(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"strips inline tags", "A <i>great</i> <b>show</b>.", "A great show."},
		{"br becomes newline", "Line one.<br>Line two.", "Line one.\nLine two."},
		{"self-closing br", "Line one.<br />Line two.", "Line one.\nLine two."},
		{"decodes entities", "Tom &amp; Jerry &quot;quoted&quot;", `Tom & Jerry "quoted"`},
		{"collapses blank runs and trims", "  Para one.<br><br><br>Para two.  ", "Para one.\n\nPara two."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cleanAniListDescription(tt.in))
		})
	}
}

package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

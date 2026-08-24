package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScoreRelevance covers every bucket scoreRelevance returns, in priority
// order. A regression that flips two buckets (e.g. prefix > word-prefix) is
// not visually obvious in the search UI — the right title just slips a few
// rows down — so this table is the authoritative ranking spec.
func TestScoreRelevance(t *testing.T) {
	cases := []struct {
		name      string
		nameField string
		search    string
		want      int
	}{
		{"exact match wins", "Naruto", "naruto", relevanceExact},
		{"word-exact beats prefix", "Naruto Shippuden", "naruto", relevanceWordExact},
		{"prefix when no whole word matches", "Narutopedia", "naruto", relevancePrefix},
		{"word-prefix when not at start and no whole-word hit", "The Narutoverse Adventure", "naru", relevanceWordPrefix},
		{"contains for substring inside a word", "Inarutorial", "naru", relevanceContains},
		{"fall-through to FTS bucket when nothing matches at all", "One Piece", "naruto", relevanceFTS},
		{"case is normalized on the name side too", "NARUTO", "naruto", relevanceExact},
		{"empty search → nothing matches as exact (empty == empty short-circuits)", "", "", relevanceExact},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := scoreRelevance(c.nameField, strings.ToLower(c.search))
			assert.Equal(t, c.want, got)
		})
	}
}

func TestScoreRelevance_OrderingInvariant(t *testing.T) {
	// Concrete invariant the search relies on: lower score = better match.
	// If anyone inverts the comparator, this trips before it ships.
	assert.Less(t, relevanceExact, relevanceWordExact)
	assert.Less(t, relevanceWordExact, relevancePrefix)
	assert.Less(t, relevancePrefix, relevanceWordPrefix)
	assert.Less(t, relevanceWordPrefix, relevanceContains)
	assert.Less(t, relevanceContains, relevanceFTS)
	assert.Less(t, relevanceFTS, relevanceFuzzy)
}

package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Soviann/trackarr/internal/model"
)

// CombineMergedStatus reconciles the statuses of two titles being merged into
// one. "older"/"newest" refer to which title owns the lower/higher season
// numbers after the merge offset is applied. The full 4×4 grid below is the
// product spec agreed with the PO.
func TestCombineMergedStatus(t *testing.T) {
	P := model.TitleStatusPlanToWatch
	W := model.TitleStatusWatching
	C := model.TitleStatusCompleted
	D := model.TitleStatusDropped

	cases := []struct {
		older, newest, want model.TitleStatus
	}{
		// older = plan_to_watch
		{P, P, P},
		{P, W, W},
		{P, C, C},
		{P, D, D},
		// older = watching
		{W, P, W},
		{W, W, W},
		{W, C, C},
		{W, D, D},
		// older = completed — finishing the old block but a newer unstarted
		// season means there is fresh content, so plan_to_watch bumps to watching.
		{C, P, W},
		{C, W, W},
		{C, C, C},
		{C, D, D},
		// older = dropped — dropped is sticky.
		{D, P, D},
		{D, W, D},
		{D, C, D},
		{D, D, D},
	}

	for _, tc := range cases {
		got := model.CombineMergedStatus(tc.older, tc.newest)
		assert.Equalf(t, tc.want, got, "CombineMergedStatus(%q, %q)", tc.older, tc.newest)
	}
}

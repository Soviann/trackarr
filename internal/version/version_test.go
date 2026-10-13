package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfo_NominalWithCommitAndDate(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, BuildDate
	defer func() {
		Version, Commit, BuildDate = origVersion, origCommit, origDate
	}()

	Version = "v1.26.16"
	Commit = "abc1234"
	BuildDate = "2026-10-13T16:00:00Z"

	expected := "v1.26.16 (abc1234, 2026-10-13T16:00:00Z)"
	assert.Equal(t, expected, Info())
}

func TestInfo_NominalDefaultValues(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, BuildDate
	defer func() {
		Version, Commit, BuildDate = origVersion, origCommit, origDate
	}()

	Version = "dev"
	Commit = "none"
	BuildDate = "unknown"

	assert.Equal(t, "dev", Info())
}

func TestInfo_BoundaryCommitWithoutDate(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, BuildDate
	defer func() {
		Version, Commit, BuildDate = origVersion, origCommit, origDate
	}()

	tests := []struct {
		name     string
		version  string
		commit   string
		date     string
		expected string
	}{
		{
			name:     "commit present but date unknown",
			version:  "v1.26.16",
			commit:   "abc1234",
			date:     "unknown",
			expected: "v1.26.16 (abc1234)",
		},
		{
			name:     "commit present but date empty",
			version:  "v1.26.16",
			commit:   "abc1234",
			date:     "",
			expected: "v1.26.16 (abc1234)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.version
			Commit = tc.commit
			BuildDate = tc.date
			assert.Equal(t, tc.expected, Info())
		})
	}
}

func TestInfo_BoundaryNoCommit(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, BuildDate
	defer func() {
		Version, Commit, BuildDate = origVersion, origCommit, origDate
	}()

	tests := []struct {
		name     string
		version  string
		commit   string
		date     string
		expected string
	}{
		{
			name:     "commit is none",
			version:  "v1.26.16",
			commit:   "none",
			date:     "unknown",
			expected: "v1.26.16",
		},
		{
			name:     "commit is empty string",
			version:  "dev",
			commit:   "",
			date:     "",
			expected: "dev",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.version
			Commit = tc.commit
			BuildDate = tc.date
			assert.Equal(t, tc.expected, Info())
		})
	}
}

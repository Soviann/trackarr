package cmd

import (
	"fmt"

	"github.com/Soviann/trackarr/internal/version"
)

// Version returns the application version string.
func Version() string {
	return fmt.Sprintf("Trackarr %s", version.Info())
}

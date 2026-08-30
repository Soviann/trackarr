package version

var (
	// Version is the semantic version of the application. Overridden via -ldflags at build time.
	Version = "v1.8.0"
	// Commit is the git commit SHA. Overridden via -ldflags at build time.
	Commit = "none"
	// BuildDate is the date and time of the build. Overridden via -ldflags at build time.
	BuildDate = "unknown"
)

// Info returns formatted version information.
func Info() string {
	if Commit != "none" && Commit != "" {
		return Version + " (" + Commit + ", " + BuildDate + ")"
	}
	return Version
}

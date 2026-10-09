package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/version"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Stdio Hygiene: Stdio MCP transports use stdin/stdout for JSON-RPC messages.
	// Any non-protocol output on stdout corrupts the communication frame.
	// Enforce all logs to stderr immediately at startup.
	log.SetOutput(os.Stderr)

	var (
		rawURL     string
		apiKey     string
		timeoutSec int
		showVer    bool
	)

	flag.StringVar(&rawURL, "url", "", "Trackarr server base URL (or env TRACKARR_URL)")
	flag.StringVar(&apiKey, "api-key", "", "Trackarr secret API key (or env TRACKARR_API_KEY)")
	flag.IntVar(&timeoutSec, "timeout", 30, "HTTP request timeout in seconds")
	flag.BoolVar(&showVer, "version", false, "Print version and exit")
	flag.BoolVar(&showVer, "v", false, "Print version and exit (shorthand)")

	flag.Parse()

	if showVer {
		fmt.Fprintf(os.Stderr, "trackarr-mcp version %s\n", version.Info())
		os.Exit(0)
	}

	// Environment variable fallback
	if rawURL == "" {
		rawURL = os.Getenv("TRACKARR_URL")
	}
	if rawURL == "" {
		rawURL = "http://127.0.0.1:8080"
	}

	if apiKey == "" {
		apiKey = os.Getenv("TRACKARR_API_KEY")
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: TRACKARR_API_KEY is required (set TRACKARR_API_KEY env var or -api-key flag)")
		os.Exit(1)
	}

	client := NewClient(rawURL, apiKey, time.Duration(timeoutSec)*time.Second)

	mcpServer := server.NewMCPServer(
		"trackarr",
		version.Version,
		server.WithResourceCapabilities(false, false),
		server.WithToolCapabilities(false),
	)

	registerTools(mcpServer, client)
	registerResources(mcpServer, client)

	log.Printf("trackarr-mcp starting stdio transport (target: %s, version: %s)", strings.TrimRight(rawURL, "/"), version.Version)
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: plextracker <serve|import|migrate>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Println("PlexTracker starting...")
	case "import":
		fmt.Println("Import not yet implemented")
	case "migrate":
		fmt.Println("Migrate not yet implemented")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

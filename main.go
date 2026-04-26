package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/nicolasvasse/plextracker/cmd"
)

//go:embed frontend/dist
var distFS embed.FS

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: plextracker <serve|import|migrate|backfill-accents>")
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = cmd.Serve(distFS)
	case "import":
		err = cmd.Import(os.Args[2:])
	case "migrate":
		fmt.Println("Migrate not yet implemented")
	case "backfill-accents":
		err = cmd.BackfillAccents(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}
}

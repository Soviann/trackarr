package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/Soviann/trackarr/cmd"
)

//go:embed frontend/dist
var distFS embed.FS

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: trackarr <serve|import|backfill-accents|reset-password|version>")
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
	case "reset-password":
		err = cmd.ResetPassword(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(cmd.Version())
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}
}

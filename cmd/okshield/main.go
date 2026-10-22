// Command okshield is a lightweight TUI for Docker inspection, OpenTelemetry
// ingestion and deterministic incident diagnosis.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/CarlosArguelloDev/okshield/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print build information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	// The Bubble Tea program will be wired here in upcoming commits.
	fmt.Fprintln(os.Stderr, "okshield: TUI not implemented yet (use -version)")
	os.Exit(1)
}

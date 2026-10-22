// Command okshield is a lightweight TUI for Docker inspection, OpenTelemetry
// ingestion and deterministic incident diagnosis.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/tui"
	"github.com/CarlosArguelloDev/okshield/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print build information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "okshield:", err)
		os.Exit(1)
	}
}

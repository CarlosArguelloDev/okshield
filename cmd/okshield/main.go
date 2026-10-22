// Command okshield is a lightweight TUI for Docker inspection, OpenTelemetry
// ingestion and deterministic incident diagnosis.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/tui"
	"github.com/CarlosArguelloDev/okshield/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print build information and exit")
	dockerHost := flag.String("docker-host", os.Getenv("DOCKER_HOST"), "Docker daemon endpoint (unix://, tcp://, npipe://)")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := docker.NewClient(*dockerHost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "okshield:", err)
		os.Exit(1)
	}
	defer cli.Close()

	p := tea.NewProgram(tui.New(tui.Options{
		Ctx:    ctx,
		Docker: cli,
	}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "okshield:", err)
		os.Exit(1)
	}
}

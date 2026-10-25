// Command okshield is a lightweight TUI for Docker infrastructure inspection,
// deterministic log clustering and incident diagnosis.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/diagnose"
	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
	"github.com/CarlosArguelloDev/okshield/internal/tui"
	"github.com/CarlosArguelloDev/okshield/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print build information and exit")
	oneShot := flag.Bool("pulse", false, "run deterministic /whatsup + /diagnose report once and exit")
	dockerHost := flag.String("docker-host", os.Getenv("DOCKER_HOST"), "Docker daemon endpoint (unix://, tcp://, npipe://)")
	langFlag := flag.String("lang", envOr("OKSHIELD_LANG", "es"), "UI language (es|en)")
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

	if *oneShot {
		if err := runOneShot(ctx, cli); err != nil {
			fmt.Fprintln(os.Stderr, "okshield:", err)
			os.Exit(1)
		}
		return
	}

	uiLang, _ := tui.ParseLang(*langFlag)
	p := tea.NewProgram(tui.New(tui.Options{
		Ctx:    ctx,
		Docker: cli,
		Lang:   uiLang,
	}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "okshield:", err)
		os.Exit(1)
	}
}

func runOneShot(ctx context.Context, cli *docker.Client) error {
	containers, err := cli.ListContainers(ctx)
	if err != nil {
		return err
	}
	inputs := make([]summarizer.ContainerInput, 0, len(containers))
	for _, c := range containers {
		d, err := cli.InspectContainer(ctx, c.ID)
		if err != nil {
			continue
		}
		var st docker.Stats
		if d.Running {
			st, _ = cli.ContainerStats(ctx, c.ID)
		}
		logs, _ := cli.ContainerLogs(ctx, c.ID, 200)
		inputs = append(inputs, summarizer.ContainerInput{
			Detail: d,
			Stats:  st,
			Logs:   logs,
		})
	}
	rep := summarizer.Analyze(inputs, 5)
	fmt.Printf("=== OKSHIELD /WHATSUP (%d total: %d running, %d degraded, %d failed) ===\n",
		rep.Total, rep.Running, rep.Degraded, rep.Failed)
	for _, cp := range rep.Containers {
		fmt.Printf("- %s [%s] healthy=%v anomalies=%v\n", cp.Name, cp.State, cp.Healthy, cp.Anomalies)
		for _, cl := range cp.TopClusters {
			fmt.Printf("    [%s] x%d: %s\n", cl.Severity, cl.Count, cl.Template)
		}
	}
	fmt.Println("\n=== OKSHIELD /DIAGNOSE ===")
	for _, in := range inputs {
		diag := diagnose.Classify(in)
		fmt.Printf("- %s -> %s [%s] (conf=%.0f%%)\n  What: %s\n  Fix:  %s\n",
			diag.Container, diag.Category, diag.FaultLabel, diag.Confidence*100,
			diag.WhatHappened, firstOrEmpty(diag.Runbook))
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

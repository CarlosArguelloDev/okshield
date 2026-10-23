package tui

import (
	"fmt"
	"strings"

	"github.com/CarlosArguelloDev/okshield/internal/diagnose"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
	"github.com/CarlosArguelloDev/okshield/internal/telemetry"
)

func (m Model) renderPulse(maxLines int) string {
	s := m.styles
	tr := m.tr()
	if !m.hasPulse {
		return s.Muted.Render(tr.PulseHint)
	}
	rep := m.pulse
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s %d   %s %d   %s %d\n\n",
		s.Title.Render(tr.PulseTitle),
		s.StatusBadge(HealthOK, GlyphRunning), rep.Running,
		s.StatusBadge(HealthDegraded, GlyphDegraded), rep.Degraded,
		s.StatusBadge(HealthError, GlyphDown), rep.Failed,
	)

	if len(rep.Containers) == 0 {
		b.WriteString(s.Muted.Render(tr.PulseNoContainers))
		return b.String()
	}

	containers := rep.Containers
	if sel, ok := m.selectedContainer(); ok {
		filtered := make([]summarizer.ContainerPulse, 0, 1)
		for _, cp := range rep.Containers {
			if cp.Name == sel.Name {
				filtered = append(filtered, cp)
				break
			}
		}
		containers = filtered
	}

	for idx, cp := range containers {
		if idx > 0 {
			b.WriteByte('\n')
		}
		badge := s.StatusBadge(HealthOK, GlyphRunning)
		if !cp.Healthy {
			if cp.OOMKilled {
				badge = s.StatusBadge(HealthError, GlyphError)
			} else if cp.State == "exited" || cp.ExitCode != 0 {
				badge = s.StatusBadge(HealthError, GlyphDown)
			} else {
				badge = s.StatusBadge(HealthDegraded, GlyphDegraded)
			}
		}
		fmt.Fprintf(&b, "%s  %s  %s\n",
			badge,
			s.Selected.Render(fmt.Sprintf("%-16s", cp.Name)),
			s.Muted.Render(fmt.Sprintf("CPU %4.1f%% · RAM %4.1f%% · logs %-3d (err:%d warn:%d)",
				cp.CPUPercent, cp.MemPercent, cp.TotalLines, cp.ErrorCount, cp.WarnCount)),
		)

		for _, a := range cp.Anomalies {
			b.WriteString("   " + s.StatusBadge(HealthDegraded, GlyphDegraded) + " " + s.Warn.Render(a) + "\n")
		}
		for _, cl := range cp.TopClusters {
			if cl.Severity == summarizer.SevInfo && len(cp.Anomalies) == 0 && cp.ErrorCount > 0 {
				continue
			}
			b.WriteString("   " + m.formatCluster(cl) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderLogs(maxLines int) string {
	s := m.styles
	tr := m.tr()
	var b strings.Builder

	c, ok := m.selectedContainer()
	if !ok {
		b.WriteString(s.Muted.Render(tr.LogsSelectHint))
		return b.String()
	}

	fmt.Fprintf(&b, "%s %s %s\n",
		s.Title.Render(tr.LogsTitle),
		s.Text.Render(c.Name),
		s.Muted.Render(fmt.Sprintf(tr.LogsRingCount, len(m.logs), len(m.clusters))),
	)

	if len(m.clusters) == 0 {
		b.WriteString(s.Muted.Render(tr.LogsEmpty))
		return b.String()
	}

	for i := 0; i < len(m.clusters); i++ {
		b.WriteString("  " + m.formatCluster(m.clusters[i]) + "\n")
	}

	b.WriteString("\n" + s.Title.Render(tr.LogsRecent) + "\n")
	for _, line := range m.logs {
		b.WriteString("  " + s.Muted.Render(truncate(line, max(20, m.width-40))) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderDiagnose() string {
	s := m.styles
	tr := m.tr()
	diag := m.diag
	if !m.hasDiag {
		// Automatically classify currently inspected container if available.
		if c, ok := m.selectedContainer(); ok && m.detail.ID == c.ID {
			diag = diagnose.Classify(summarizer.ContainerInput{
				Detail: m.detail,
				Stats:  m.stats,
				Logs:   m.logs,
			})
		} else {
			return s.Muted.Render(tr.DiagHint)
		}
	}

	var b strings.Builder
	diagHealth := HealthError
	if diag.Category == diagnose.CatHealthy {
		diagHealth = HealthOK
	} else if diag.Category == diagnose.CatCPUThrottling {
		diagHealth = HealthDegraded
	}
	fmt.Fprintf(&b, "%s  %s  %s\n\n",
		s.Title.Render(tr.DiagTitle+diag.Container),
		s.StatusBadge(diagHealth, fmt.Sprintf(" %s · %s ", diag.Category, diag.FaultLabel)),
		s.Muted.Render(fmt.Sprintf(tr.DiagConfidence, diag.Confidence*100)),
	)

	b.WriteString(s.Selected.Render(tr.DiagWhat) + " " + s.Text.Render(diag.WhatHappened) + "\n")
	b.WriteString(s.Selected.Render(tr.DiagWhy) + " " + s.Text.Render(diag.WhyItHappened) + "\n\n")

	b.WriteString(s.Title.Render(tr.DiagTreeTitle) + "\n")
	for _, node := range diag.RootCauseTree {
		b.WriteString("  " + s.Warn.Render(node) + "\n")
	}

	b.WriteString("\n" + s.Title.Render(tr.DiagRunbook) + "\n")
	for i, cmd := range diag.Runbook {
		fmt.Fprintf(&b, "  %s %s\n", s.Selected.Render(fmt.Sprintf("%d.", i+1)), s.Text.Render(cmd))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderOtelStatus() string {
	s := m.styles
	tr := m.tr()
	var st telemetry.StoreStats
	if m.receiver != nil {
		st = m.receiver.Stats()
	} else if m.store != nil {
		st = m.store.Stats(telemetry.DefaultOTLPGRPCAddr, telemetry.DefaultOTLPHTTPAddr, false)
	} else {
		st = telemetry.StoreStats{
			RingCap:     telemetry.DefaultRingCapacity,
			MemoryLimit: "64MB",
			Threads:     2,
			GRPCAddr:    telemetry.DefaultOTLPGRPCAddr,
			HTTPAddr:    telemetry.DefaultOTLPHTTPAddr,
		}
	}

	var b strings.Builder
	b.WriteString(s.Title.Render(tr.OtelTitle) + "\n\n")
	k := kv{b: &b, s: s}
	stateStr := s.StatusBadge(HealthError, GlyphDown) + " " + s.Warn.Render(tr.OtelInactive)
	if st.Listening {
		stateStr = s.StatusBadge(HealthOK, GlyphOK) + " " + s.OK.Render(tr.OtelListening)
	}
	k.row(tr.OtelReceiver, stateStr)
	k.row("OTLP gRPC", st.GRPCAddr)
	k.row("OTLP HTTP", st.HTTPAddr+" (/v1/logs, /v1/metrics)")
	k.row("ring buffer", fmt.Sprintf(tr.OtelRing, st.RingLen, st.RingCap, st.TotalIngested))
	k.row("batch writer", fmt.Sprintf(tr.OtelBatch, st.BatchesFlushed))
	k.row("DuckDB rows", fmt.Sprintf(tr.OtelRows, st.StoredRows))
	k.row(tr.OtelSize, humanBytes(uint64(max(0, st.DiskBytes))))
	k.row(tr.OtelLimits, fmt.Sprintf("memory_limit=%s · threads=%d · TTL=%s", st.MemoryLimit, st.Threads, st.Retention))
	b.WriteString("\n" + s.Muted.Render(tr.HelpFooterAny))
	return s.Overlay.Render(b.String())
}

func (m Model) formatCluster(cl summarizer.LogCluster) string {
	s := m.styles
	var badge string
	var count string
	switch cl.Severity {
	case summarizer.SevError:
		badge = s.StatusBadge(HealthError, GlyphError)
		count = s.Err.Render(fmt.Sprintf("x%d:", cl.Count))
	case summarizer.SevWarn:
		badge = s.StatusBadge(HealthDegraded, GlyphDegraded)
		count = s.Warn.Render(fmt.Sprintf("x%d:", cl.Count))
	default:
		badge = s.StatusBadge(HealthInfo, GlyphInfo)
		count = s.OK.Render(fmt.Sprintf("x%d:", cl.Count))
	}
	return badge + " " + count + " " + s.Text.Render(cl.Template)
}

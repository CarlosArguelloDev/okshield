package tui

import (
	"fmt"
	"strings"

	zone "github.com/lrstanley/bubblezone"

	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
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

	clusterIdx := 0
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
			b.WriteString("   " + m.formatCluster(clusterIdx, cl) + "\n")
			clusterIdx++
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
		b.WriteString("  " + m.formatCluster(i, m.clusters[i]) + "\n")
	}

	b.WriteString("\n" + s.Title.Render(tr.LogsRecent) + "\n")
	for _, line := range m.logs {
		b.WriteString("  " + s.Muted.Render(line) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) formatCluster(idx int, cl summarizer.LogCluster) string {
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
	return zone.Mark(fmt.Sprintf("cluster-%d", idx), badge) + " " + count + " " + s.Text.Render(cl.Template)
}

func (m Model) visibleClusters() []summarizer.LogCluster {
	if m.tab == TabLogs {
		return m.clusters
	}
	if !m.hasPulse {
		return nil
	}
	containers := m.pulse.Containers
	if sel, ok := m.selectedContainer(); ok {
		for _, cp := range m.pulse.Containers {
			if cp.Name == sel.Name {
				containers = []summarizer.ContainerPulse{cp}
				break
			}
		}
	}
	var out []summarizer.LogCluster
	for _, cp := range containers {
		for _, cl := range cp.TopClusters {
			if cl.Severity == summarizer.SevInfo && len(cp.Anomalies) == 0 && cp.ErrorCount > 0 {
				continue
			}
			out = append(out, cl)
		}
	}
	return out
}

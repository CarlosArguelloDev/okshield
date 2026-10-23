package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
)

// kv renders aligned "key  value" rows for the details pane.
type kv struct {
	b *strings.Builder
	s Styles
}

func (k kv) row(key, val string) {
	if val == "" {
		val = "-"
	}
	fmt.Fprintf(k.b, "%s %s\n", k.s.Muted.Render(fmt.Sprintf("%-12s", key)), k.s.Text.Render(val))
}

func (k kv) section(title string) {
	k.b.WriteString("\n" + k.s.Title.Render(title) + "\n")
}

// renderMetrics renders the dedicated Métricas tab with a large multi-row
// Unicode Braille chart and interactive metric selector (CPU, Memoria, Red, Disco).
func (m Model) renderMetrics(bodyRows int) string {
	s := m.styles
	var b strings.Builder

	target, hasTarget := m.selectedContainer()
	if !hasTarget {
		for _, c := range m.containers {
			if c.State == "running" {
				target = c
				hasTarget = true
				break
			}
		}
		if !hasTarget && len(m.containers) > 0 {
			target = m.containers[0]
			hasTarget = true
		}
	}
	if !hasTarget {
		return s.Muted.Render("Sin contenedores activos para graficar métricas en tiempo real.")
	}

	h := containerHealth(target)
	badge := s.StatusGlyph(h)
	if h == HealthError {
		badge = s.StatusBadge(HealthError, GlyphDown)
	}
	fmt.Fprintf(&b, "%s %s  %s\n",
		badge,
		s.Title.Render("Métricas en Tiempo Real · "+target.Name),
		s.Muted.Render("(↑/↓ con foco o 'm' cambia gráfica)"),
	)

	// Metric selector pills.
	var pills []string
	for k := MetricKind(0); k < metricKindCount; k++ {
		if k == m.metricKind {
			pills = append(pills, s.StatusBadge(HealthInfo, " ▸ "+k.String()+" "))
		} else {
			pills = append(pills, s.Muted.Render("["+k.String()+"]"))
		}
	}
	b.WriteString(strings.Join(pills, "  ") + "\n\n")

	var cpuHist, memHist, netHist, diskHist []float64
	var rxRate, txRate float64
	if ser, ok := m.history[target.ID]; ok {
		cpuHist = ser.Samples(MetricCPU, make([]float64, 0, HistoryCapacity))
		memHist = ser.Samples(MetricMem, make([]float64, 0, HistoryCapacity))
		netHist = ser.Samples(MetricNet, make([]float64, 0, HistoryCapacity))
		diskHist = ser.Samples(MetricDisk, make([]float64, 0, HistoryCapacity))
		rxRate, txRate = ser.NetRates()
	}
	if len(cpuHist) == 0 && m.stats.CPUPercent > 0 {
		cpuHist = []float64{m.stats.CPUPercent}
		memHist = []float64{m.stats.MemPercent}
	}

	var activeSeries []float64
	maxVal := 100.0
	chartStyle := s.Title
	switch m.metricKind {
	case MetricMem:
		activeSeries = memHist
		chartStyle = s.Key
	case MetricNet:
		activeSeries = netHist
		maxVal = maxSeriesVal(netHist, 1024.0)
		chartStyle = s.OK
	case MetricDisk:
		activeSeries = diskHist
		maxVal = maxSeriesVal(diskHist, 1024.0)
		chartStyle = s.Warn
	default:
		activeSeries = cpuHist
		chartStyle = s.Title
	}

	sideW := max(minSidebarW, m.width/6)
	remW := max(20, m.width-sideW)
	inspectorW := max(20, remW-max(24, remW*26/100))
	chartW := clampInt(inspectorW-18, 24, 54)
	chartH := clampInt(bodyRows-10, 6, 11)

	rows := BrailleChart(activeSeries, chartW, chartH, maxVal)
	for r, line := range rows {
		var yLabel string
		switch {
		case r == 0:
			yLabel = FormatMetricAxis(m.metricKind, maxVal)
		case r == chartH/2:
			yLabel = FormatMetricAxis(m.metricKind, maxVal*0.5)
		case r == chartH-1:
			yLabel = FormatMetricAxis(m.metricKind, 0)
		default:
			yLabel = "       "
		}
		fmt.Fprintf(&b, "%s %s%s\n",
			s.Muted.Render(yLabel),
			s.Muted.Render("┤"),
			chartStyle.Render(line),
		)
	}
	xAxis := "       └" + strings.Repeat("─", chartW)
	b.WriteString(s.Muted.Render(xAxis) + "\n")
	timeSpan := max(4, chartW-10)
	b.WriteString(s.Muted.Render("        -60s"+strings.Repeat(" ", timeSpan)+"ahora") + "\n\n")

	memLim := m.stats.MemLimit
	if memLim == 0 && m.detail.ID == target.ID && m.detail.MemoryLimit > 0 {
		memLim = uint64(m.detail.MemoryLimit)
	}

	k := kv{b: &b, s: s}
	k.row(metricCursor(m.metricKind == MetricCPU, "CPU Usage"), fmt.Sprintf("%s %5.1f%%  %s  %s",
		m.cpuBar.ViewAs(clamp01(m.stats.CPUPercent/100.0)),
		m.stats.CPUPercent,
		s.Title.Render(BrailleSparkline(cpuHist, 16, 100.0)),
		s.Muted.Render(fmt.Sprintf("throt:%d", m.stats.Throttled)),
	))
	k.row(metricCursor(m.metricKind == MetricMem, "Memory Usage"), fmt.Sprintf("%s %s / %s (%4.1f%%)  %s",
		m.memBar.ViewAs(clamp01(m.stats.MemPercent/100.0)),
		humanBytes(m.stats.MemUsage),
		humanBytes(memLim),
		m.stats.MemPercent,
		s.Key.Render(BrailleSparkline(memHist, 16, 100.0)),
	))
	k.row(metricCursor(m.metricKind == MetricNet, "Red (RX/TX)"), fmt.Sprintf("↓ %s (%s/s) · ↑ %s (%s/s)  %s",
		humanBytes(m.stats.NetRxBytes),
		humanBytes(uint64(rxRate)),
		humanBytes(m.stats.NetTxBytes),
		humanBytes(uint64(txRate)),
		s.OK.Render(BrailleSparkline(netHist, 16, 0)),
	))
	k.row(metricCursor(m.metricKind == MetricDisk, "Disco I/O"), fmt.Sprintf("lect %s · escr %s · PIDs %d  %s",
		humanBytes(m.stats.BlkReadBytes),
		humanBytes(m.stats.BlkWriteBytes),
		m.stats.PIDs,
		s.Warn.Render(BrailleSparkline(diskHist, 16, 0)),
	))
	return strings.TrimRight(b.String(), "\n")
}

func metricCursor(active bool, label string) string {
	if active {
		return "▸ " + label
	}
	return "  " + label
}

func maxSeriesVal(xs []float64, floor float64) float64 {
	m := floor
	for _, v := range xs {
		if v > m {
			m = v
		}
	}
	return m
}

// renderDetails renders the Detalles tab for the current selection.
func (m Model) renderDetails() string {
	if m.itemCount() == 0 {
		return m.styles.Muted.Render("sin selección")
	}
	var b strings.Builder
	k := kv{b: &b, s: m.styles}
	switch m.view {
	case ViewNetworks:
		m.networkDetails(k, m.networks[m.cursor])
	case ViewVolumes:
		m.volumeDetails(k, m.volumes[m.cursor])
	default:
		if c, ok := m.selectedContainer(); ok {
			m.containerDetails(k, c)
		} else {
			m.generalDetails(k)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) generalDetails(k kv) {
	s := m.styles
	k.b.WriteString(s.StatusBadge(HealthInfo, " ALL ") + " " + s.Title.Render("Resumen General de Infraestructura") + "\n")
	k.row("host", m.hostname)
	k.row("contenedores", strconv.Itoa(len(m.containers)))
	k.row("redes", strconv.Itoa(len(m.networks)))
	k.row("volúmenes", strconv.Itoa(len(m.volumes)))

	if len(m.containers) > 0 {
		k.section("Contenedores")
		for _, c := range m.containers {
			k.row(c.Name, c.Image+" · "+c.Status)
		}
	}
}

func (m Model) containerDetails(k kv, c docker.Container) {
	s := m.styles
	h := containerHealth(c)
	badge := s.StatusGlyph(h)
	if h == HealthError {
		badge = s.StatusBadge(HealthError, GlyphDown)
	}
	k.b.WriteString(badge + " " + s.Title.Render(c.Name) + s.Muted.Render("  "+c.ShortID()) + "\n")
	k.row("imagen", c.Image)
	k.row("estado", c.Status)

	d := m.detail
	if d.ID == c.ID {
		if d.OOMKilled {
			k.row("OOMKilled", s.StatusBadge(HealthError, GlyphError)+" sí")
		}
		if !d.Running && !d.StartedAt.IsZero() {
			k.row("exit code", strconv.Itoa(d.ExitCode))
		}
		if d.Error != "" {
			k.row("error", d.Error)
		}
		k.row("reinicios", fmt.Sprintf("%d (política: %s)", d.RestartCount, orDash(d.RestartPolicy)))
		if d.Health != "" {
			k.row("health", d.Health)
		}
		if d.Running && !d.StartedAt.IsZero() {
			k.row("uptime", time.Since(d.StartedAt).Truncate(time.Second).String())
		}
		k.row("límite RAM", limitStr(d.MemoryLimit, humanBytes(uint64(d.MemoryLimit))))
		k.row("límite CPU", limitStr(d.NanoCPUs, fmt.Sprintf("%.2f cpu", float64(d.NanoCPUs)/1e9)))
		k.row("cmd", strings.Join(d.Cmd, " "))
		k.row("env vars", strconv.Itoa(d.EnvCount))
	} else if m.detailErr != nil {
		k.row("inspect", s.Err.Render(m.detailErr.Error()))
	}

	if len(c.Ports) > 0 {
		k.section("Puertos")
		for _, p := range c.Ports {
			if p.Public != 0 {
				k.b.WriteString(fmt.Sprintf("  %s:%d → %d/%s\n", orDash(p.IP), p.Public, p.Private, p.Proto))
			} else {
				k.b.WriteString(fmt.Sprintf("  %d/%s (no publicado)\n", p.Private, p.Proto))
			}
		}
	}
	if len(c.Networks) > 0 {
		k.section("Redes")
		for _, n := range c.Networks {
			k.row(n.Network, n.IP)
		}
	}
	if len(c.Mounts) > 0 {
		k.section("Mounts")
		for _, mt := range c.Mounts {
			src := mt.Source
			if mt.Type == "volume" && mt.Name != "" {
				src = mt.Name
			}
			mode := "rw"
			if !mt.RW {
				mode = "ro"
			}
			k.b.WriteString(fmt.Sprintf("  [%s] %s → %s (%s)\n", mt.Type, src, mt.Destination, mode))
		}
	}
}

func (m Model) networkDetails(k kv, n docker.Network) {
	k.b.WriteString(m.styles.StatusBadge(HealthInfo, " NET ") + " " + m.styles.Title.Render(n.Name) + "\n")
	k.row("driver", n.Driver)
	k.row("scope", n.Scope)
	k.row("interna", strconv.FormatBool(n.Internal))
	for _, sn := range n.Subnets {
		k.row("subnet", sn.CIDR+"  gw "+orDash(sn.Gateway))
	}
	k.section("Contenedores conectados")
	found := false
	for _, c := range m.containers {
		for _, a := range c.Networks {
			if a.Network == n.Name {
				k.row(c.Name, a.IP)
				found = true
			}
		}
	}
	if !found {
		k.b.WriteString(m.styles.Muted.Render("  ninguno") + "\n")
	}
}

func (m Model) volumeDetails(k kv, v docker.Volume) {
	k.b.WriteString(m.styles.StatusBadge(HealthInfo, " VOL ") + " " + m.styles.Title.Render(v.Name) + "\n")
	k.row("driver", v.Driver)
	k.row("scope", v.Scope)
	k.row("mountpoint", v.Mountpoint)
	k.row("creado", v.CreatedAt)
	k.section("Usado por")
	found := false
	for _, c := range m.containers {
		for _, mt := range c.Mounts {
			if mt.Type == "volume" && mt.Name == v.Name {
				k.row(c.Name, mt.Destination)
				found = true
			}
		}
	}
	if !found {
		k.b.WriteString(m.styles.Muted.Render("  ningún contenedor (huérfano)") + "\n")
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func limitStr(v int64, formatted string) string {
	if v <= 0 {
		return "sin límite"
	}
	return formatted
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

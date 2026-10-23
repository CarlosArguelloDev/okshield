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

// renderDetails renders the Métricas/Detalles tab for the current selection.
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

	var cpuBuf, memBuf [HistoryCapacity]float64
	for _, c := range m.containers {
		if ser, ok := m.history[c.ID]; ok {
			cs := ser.CPUSamples(make([]float64, 0, HistoryCapacity))
			ms := ser.MemSamples(make([]float64, 0, HistoryCapacity))
			k.section("Métricas en Tiempo Real · " + c.Name)
			k.row("CPU Usage", fmt.Sprintf("%s %5.1f%%  %s",
				m.cpuBar.ViewAs(clamp01(lastVal(cs)/100.0)),
				lastVal(cs),
				s.Title.Render(BrailleSparkline(cs, 20, 100.0)),
			))
			k.row("Mem Usage", fmt.Sprintf("%s %5.1f%%  %s",
				m.memBar.ViewAs(clamp01(lastVal(ms)/100.0)),
				lastVal(ms),
				s.Key.Render(BrailleSparkline(ms, 20, 100.0)),
			))
			_ = cpuBuf
			_ = memBuf
			break
		}
	}

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

	var cpuHist, memHist []float64
	var rxRate, txRate float64
	if ser, ok := m.history[c.ID]; ok {
		cpuHist = ser.CPUSamples(make([]float64, 0, HistoryCapacity))
		memHist = ser.MemSamples(make([]float64, 0, HistoryCapacity))
		rxRate, txRate = ser.NetRates()
	}
	if len(cpuHist) == 0 && m.detail.ID == c.ID && m.detail.Running {
		cpuHist = []float64{m.stats.CPUPercent}
		memHist = []float64{m.stats.MemPercent}
	}

	memLim := m.stats.MemLimit
	if memLim == 0 && m.detail.ID == c.ID && m.detail.MemoryLimit > 0 {
		memLim = uint64(m.detail.MemoryLimit)
	}

	k.section("Métricas en Tiempo Real (Braille 60s)")
	cpuSpark := s.Title.Render(BrailleSparkline(cpuHist, 24, 100.0))
	memSpark := s.Key.Render(BrailleSparkline(memHist, 24, 100.0))
	k.row("CPU Usage", fmt.Sprintf("%s %5.1f%%  %s  %s",
		m.cpuBar.ViewAs(clamp01(m.stats.CPUPercent/100.0)),
		m.stats.CPUPercent,
		cpuSpark,
		s.Muted.Render(fmt.Sprintf("throt:%d", m.stats.Throttled)),
	))
	k.row("Memory Usage", fmt.Sprintf("%s %s / %s (%4.1f%%)  %s",
		m.memBar.ViewAs(clamp01(m.stats.MemPercent/100.0)),
		humanBytes(m.stats.MemUsage),
		humanBytes(memLim),
		m.stats.MemPercent,
		memSpark,
	))
	k.row("Red (RX/TX)", fmt.Sprintf("↓ %s (%s/s)  ·  ↑ %s (%s/s)",
		humanBytes(m.stats.NetRxBytes),
		humanBytes(uint64(rxRate)),
		humanBytes(m.stats.NetTxBytes),
		humanBytes(uint64(txRate)),
	))
	k.row("Bloque I/O", fmt.Sprintf("lectura %s  ·  escritura %s  ·  PIDs %d",
		humanBytes(m.stats.BlkReadBytes),
		humanBytes(m.stats.BlkWriteBytes),
		m.stats.PIDs,
	))
	if m.receiver != nil {
		ost := m.receiver.Stats()
		k.row("OTel Ingest", fmt.Sprintf("%d logs en ring buffer (total: %d)", ost.RingLen, ost.TotalIngested))
	}

	d := m.detail
	if d.ID == c.ID {
		k.section("Inspección de Contenedor")
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

func lastVal(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	return xs[len(xs)-1]
}

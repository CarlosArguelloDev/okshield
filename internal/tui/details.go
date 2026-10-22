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
		m.containerDetails(k, m.containers[m.cursor])
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) containerDetails(k kv, c docker.Container) {
	s := m.styles
	k.b.WriteString(s.StatusGlyph(containerHealth(c)) + " " + s.Title.Render(c.Name) + s.Muted.Render("  "+c.ShortID()) + "\n")
	k.row("imagen", c.Image)
	k.row("estado", c.Status)

	d := m.detail
	if d.ID == c.ID {
		if d.OOMKilled {
			k.row("OOMKilled", s.Err.Render("sí"))
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
		if d.Running {
			k.row("uso vivo", fmt.Sprintf("CPU %.1f%% (throt:%d) · RAM %s/%s (%.1f%%) · PIDs %d",
				m.stats.CPUPercent, m.stats.Throttled,
				humanBytes(m.stats.MemUsage), humanBytes(m.stats.MemLimit), m.stats.MemPercent,
				m.stats.PIDs))
		}
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
	k.b.WriteString(m.styles.Title.Render(n.Name) + "\n")
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
	k.b.WriteString(m.styles.Title.Render(v.Name) + "\n")
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

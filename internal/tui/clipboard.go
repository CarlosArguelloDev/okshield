package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mattn/go-isatty"
)

const toastDuration = 2 * time.Second

type clipboardCopiedMsg struct {
	label string
	btnID string
}

type clipboardClearMsg struct {
	seq int
}

type copyTarget struct {
	key   string
	label string
	value string
}

func formatOSC52(text string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
}

func copyToClipboard(text, label, btnID string) tea.Cmd {
	if text == "" {
		return nil
	}
	return func() tea.Msg {
		if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			_, _ = os.Stdout.WriteString(formatOSC52(text))
		}
		_ = clipboard.WriteAll(text)
		return clipboardCopiedMsg{label: label, btnID: btnID}
	}
}

func (m Model) copyIcon(key string) string {
	zoneID := "copy-meta-" + key
	if m.flashBtn == zoneID {
		return zone.Mark(zoneID, m.styles.OK.Render("[✓]"))
	}
	return zone.Mark(zoneID, m.styles.Muted.Render("[⧉]"))
}

func (m Model) metaCopyTargets() []copyTarget {
	switch m.view {
	case ViewNetworks:
		if m.cursor < 0 || m.cursor >= len(m.networks) {
			return nil
		}
		n := m.networks[m.cursor]
		targets := []copyTarget{
			{key: "m-net-name", label: n.Name, value: n.Name},
			{key: "d-net-name", label: n.Name, value: n.Name},
		}
		for i, sn := range n.Subnets {
			targets = append(targets, copyTarget{
				key:   fmt.Sprintf("d-subnet-%d", i),
				label: sn.CIDR,
				value: sn.CIDR,
			})
			if i == 0 {
				targets = append(targets, copyTarget{key: "m-subnet", label: sn.CIDR, value: sn.CIDR})
			}
		}
		return targets

	case ViewVolumes:
		if m.cursor < 0 || m.cursor >= len(m.volumes) {
			return nil
		}
		v := m.volumes[m.cursor]
		return []copyTarget{
			{key: "m-vol-name", label: v.Name, value: v.Name},
			{key: "d-vol-name", label: v.Name, value: v.Name},
			{key: "m-vol-mp", label: v.Mountpoint, value: v.Mountpoint},
			{key: "d-vol-mp", label: v.Mountpoint, value: v.Mountpoint},
		}

	case ViewImages:
		if m.cursor < 0 || m.cursor >= len(m.images) {
			return nil
		}
		img := m.images[m.cursor]
		return []copyTarget{
			{key: "m-img-tag", label: img.DisplayName(), value: img.DisplayName()},
			{key: "d-img-tag", label: img.DisplayName(), value: img.DisplayName()},
			{key: "m-img-id", label: img.ShortID(), value: img.ShortID()},
			{key: "d-img-id", label: img.ShortID(), value: img.ID},
		}

	default:
		c, ok := m.selectedContainer()
		if !ok {
			return nil
		}
		targets := []copyTarget{
			{key: "m-name", label: c.Name, value: c.Name},
			{key: "m-image", label: c.Image, value: c.Image},
			{key: "d-name", label: c.Name, value: c.Name},
			{key: "d-id-short", label: c.ShortID(), value: c.ShortID()},
			{key: "d-id-full", label: c.ShortID(), value: c.ID},
			{key: "d-image", label: c.Image, value: c.Image},
		}
		if len(c.Networks) > 0 {
			targets = append(targets, copyTarget{key: "m-ip", label: c.Networks[0].IP, value: c.Networks[0].IP})
		}
		if len(c.Mounts) > 0 {
			targets = append(targets, copyTarget{key: "m-mount", label: c.Mounts[0].Destination, value: c.Mounts[0].Destination})
		}
		if m.detail.ID == c.ID && len(m.detail.Cmd) > 0 {
			targets = append(targets, copyTarget{
				key:   "d-cmd",
				label: "cmd",
				value: strings.Join(m.detail.Cmd, " "),
			})
		}
		for i, p := range c.Ports {
			val := fmt.Sprintf("%d/%s", p.Private, p.Proto)
			if p.Public != 0 {
				val = fmt.Sprintf("%s:%d->%d/%s", orDash(p.IP), p.Public, p.Private, p.Proto)
			}
			targets = append(targets, copyTarget{
				key:   fmt.Sprintf("d-port-%d", i),
				label: val,
				value: val,
			})
		}
		for i, n := range c.Networks {
			targets = append(targets, copyTarget{
				key:   fmt.Sprintf("d-ip-%d", i),
				label: n.IP,
				value: n.IP,
			})
		}
		for i, mt := range c.Mounts {
			targets = append(targets, copyTarget{
				key:   fmt.Sprintf("d-mount-%d", i),
				label: mt.Destination,
				value: mt.Destination,
			})
		}
		return targets
	}
}

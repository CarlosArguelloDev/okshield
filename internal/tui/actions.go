package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const actionTimeout = 10 * time.Second

type actionSpec struct {
	id           string
	label        string
	confirmLabel string
	destructive  bool
}

func (m Model) contextActions() (title string, specs []actionSpec) {
	switch m.view {
	case ViewNetworks:
		if m.cursor >= 0 && m.cursor < len(m.networks) {
			n := m.networks[m.cursor]
			return "Actions · " + n.Name, []actionSpec{
				{id: "act-inspect", label: "[ " + IconInspect + " Inspect ]"},
				{id: "act-disconnect", label: "[ " + IconDisconnect + " Disconnect ]"},
				{id: "act-prune-net", label: "[ " + IconTrash + " Prune Unused ]", confirmLabel: "Prune Unused", destructive: true},
			}
		}
		return "Actions", nil

	case ViewVolumes:
		if m.cursor >= 0 && m.cursor < len(m.volumes) {
			v := m.volumes[m.cursor]
			return "Actions · " + v.Name, []actionSpec{
				{id: "act-inspect", label: "[ " + IconInspect + " Inspect ]"},
				{id: "act-remove-vol", label: "[ " + IconTrash + " Remove ]", confirmLabel: "Remove " + v.Name, destructive: true},
			}
		}
		return "Actions", nil

	case ViewImages:
		if m.cursor >= 0 && m.cursor < len(m.images) {
			img := m.images[m.cursor]
			return "Actions · " + img.DisplayName(), []actionSpec{
				{id: "act-pull-img", label: "[ " + IconDownload + " Pull Latest ]"},
				{id: "act-inspect", label: "[ " + IconInspect + " Inspect ]"},
				{id: "act-remove-img", label: "[ " + IconTrash + " Remove ]", confirmLabel: "Remove " + img.DisplayName(), destructive: true},
			}
		}
		return "Actions", nil

	default:
		if c, ok := m.selectedContainer(); ok {
			title = "Actions · " + c.Name
			switch c.State {
			case "running":
				return title, []actionSpec{
					{id: "act-restart", label: "[ " + IconRestart + " Restart ]"},
					{id: "act-stop", label: "[ " + IconStop + " Stop ]"},
					{id: "act-pause", label: "[ " + IconPause + " Pause ]"},
					{id: "act-shell", label: "[ " + IconShell + " Shell ]"},
					{id: "act-rm", label: "[ " + IconTrash + " Rm ]", confirmLabel: "Rm " + c.Name, destructive: true},
				}
			case "paused":
				return title, []actionSpec{
					{id: "act-unpause", label: "[ " + IconPlay + " Unpause ]"},
					{id: "act-stop", label: "[ " + IconStop + " Stop ]"},
					{id: "act-rm", label: "[ " + IconTrash + " Rm ]", confirmLabel: "Rm " + c.Name, destructive: true},
				}
			default:
				return title, []actionSpec{
					{id: "act-start", label: "[ " + IconPlay + " Start ]"},
					{id: "act-rm", label: "[ " + IconTrash + " Rm ]", confirmLabel: "Rm " + c.Name, destructive: true},
				}
			}
		}
		return "Actions", []actionSpec{
			{id: "act-whatsup", label: "[ " + IconStethoscope + " /whatsup ]"},
			{id: "act-inspect", label: "[ " + IconInspect + " Details ]"},
		}
	}
}

func (m Model) renderActionsBox(w, h int) string {
	s := m.styles
	title, specs := m.contextActions()
	title = truncate(title, max(7, w-6))

	bodyRows := max(1, h-2)
	contentW := max(1, w-4)

	var body string
	switch {
	case m.pendingConfirm != "":
		confirmBtn := zone.Mark("act-confirm", s.StatusBadge(HealthError, " "+IconCheck+" Confirm "))
		cancelBtn := zone.Mark("act-cancel", s.Key.Render("[ "+IconClose+" Cancel ]"))
		body = s.Warn.Render("Confirm "+truncate(m.pendingLabel, max(8, contentW-10))+"?") + "\n\n" + confirmBtn + "  " + cancelBtn

	case len(specs) == 0:
		body = s.Muted.Render(m.tr().NoResource)

	default:
		var lines []string
		var cur string
		curW := 0
		for _, sp := range specs {
			var styled string
			switch {
			case m.flashBtn == sp.id:
				styled = s.StatusBadge(HealthOK, " "+IconCheck+" "+strings.Trim(sp.label, "[] "))
			case sp.destructive:
				styled = s.Err.Render(sp.label)
			default:
				styled = s.Key.Render(sp.label)
			}
			btn := zone.Mark(sp.id, styled)
			bw := lipgloss.Width(styled)
			if cur == "" {
				cur = btn
				curW = bw
			} else if curW+2+bw <= contentW {
				cur += "  " + btn
				curW += 2 + bw
			} else {
				lines = append(lines, cur)
				cur = btn
				curW = bw
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		body = strings.Join(lines, "\n")
	}

	borderColor := ColorSecondary
	if m.focus == FocusMetadata {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		TopTitle:    title,
		Body:        scrollLines(body, m.metaScroll, bodyRows),
		FootBadges:  []string{"Actions", fmt.Sprintf("%d", len(specs))},
	})
}

func (m Model) triggerAction(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "act-inspect":
		m.tab = TabDetails
		m.mainScroll = 0
		m.focus = FocusMain
		return m, nil
	case "act-whatsup":
		return m.dispatch(Command{Kind: CmdWhatsup, Name: "whatsup"})
	case "act-shell":
		c, ok := m.selectedContainer()
		if !ok {
			return m, nil
		}
		cmd := exec.Command("docker", "exec", "-it", c.ID, "/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec /bin/sh; fi")
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return dockerActionMsg{label: "shell exited", btnID: "act-shell", err: err}
		})
	}

	act, ok := m.api.(ActionAPI)
	if !ok || m.api == nil {
		return m, func() tea.Msg {
			return dockerActionMsg{btnID: id, err: fmt.Errorf("docker actions unavailable")}
		}
	}

	switch id {
	case "act-start", "act-stop", "act-restart", "act-pause", "act-unpause", "act-rm":
		c, ok := m.selectedContainer()
		if !ok {
			return m, nil
		}
		return m, runDockerAction(m.ctx, id, func(ctx context.Context) (string, error) {
			switch id {
			case "act-start":
				return "Started " + c.Name, act.StartContainer(ctx, c.ID)
			case "act-stop":
				return "Stopped " + c.Name, act.StopContainer(ctx, c.ID)
			case "act-restart":
				return "Restarted " + c.Name, act.RestartContainer(ctx, c.ID)
			case "act-pause":
				return "Paused " + c.Name, act.PauseContainer(ctx, c.ID)
			case "act-unpause":
				return "Unpaused " + c.Name, act.UnpauseContainer(ctx, c.ID)
			default:
				return "Removed " + c.Name, act.RemoveContainer(ctx, c.ID, true)
			}
		})

	case "act-disconnect":
		if m.cursor < 0 || m.cursor >= len(m.networks) {
			return m, nil
		}
		n := m.networks[m.cursor]
		var targetID, targetName string
		for _, c := range m.containers {
			for _, a := range c.Networks {
				if a.Network == n.Name {
					targetID, targetName = c.ID, c.Name
					break
				}
			}
			if targetID != "" {
				break
			}
		}
		if targetID == "" {
			return m, func() tea.Msg {
				return dockerActionMsg{btnID: id, err: fmt.Errorf("no containers on %s", n.Name)}
			}
		}
		return m, runDockerAction(m.ctx, id, func(ctx context.Context) (string, error) {
			return "Disconnected " + targetName, act.DisconnectNetwork(ctx, n.ID, targetID)
		})

	case "act-prune-net":
		return m, runDockerAction(m.ctx, id, func(ctx context.Context) (string, error) {
			return "Pruned unused networks", act.PruneNetworks(ctx)
		})

	case "act-remove-vol":
		if m.cursor < 0 || m.cursor >= len(m.volumes) {
			return m, nil
		}
		v := m.volumes[m.cursor]
		return m, runDockerAction(m.ctx, id, func(ctx context.Context) (string, error) {
			return "Removed " + v.Name, act.RemoveVolume(ctx, v.Name)
		})

	case "act-pull-img":
		if m.cursor < 0 || m.cursor >= len(m.images) {
			return m, nil
		}
		img := m.images[m.cursor]
		ref := img.DisplayName()
		if ref == img.ShortID() {
			return m, func() tea.Msg {
				return dockerActionMsg{btnID: id, err: fmt.Errorf("untagged image")}
			}
		}
		return m, runDockerAction(m.ctx, id, func(ctx context.Context) (string, error) {
			return "Pulled " + ref, act.PullImage(ctx, ref)
		})

	case "act-remove-img":
		if m.cursor < 0 || m.cursor >= len(m.images) {
			return m, nil
		}
		img := m.images[m.cursor]
		return m, runDockerAction(m.ctx, id, func(ctx context.Context) (string, error) {
			return "Removed " + img.DisplayName(), act.RemoveImage(ctx, img.ID)
		})
	}
	return m, nil
}

func runDockerAction(parent context.Context, btnID string, fn func(context.Context) (string, error)) tea.Cmd {
	if parent == nil {
		parent = context.Background()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, actionTimeout)
		defer cancel()
		label, err := fn(ctx)
		return dockerActionMsg{label: label, btnID: btnID, err: err}
	}
}

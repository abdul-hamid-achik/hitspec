package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// welcomeView renders a centered first-run card shown whenever the workspace has
// no hitspec files yet. It replaces the bare empty file list with discoverable,
// keyed next steps so a fresh directory is never a dead end.
func (m model) welcomeView() string {
	s := m.styles
	w, h := m.width, max(6, m.height-4)

	title := s.accent.Render("hitspec studio")
	tagline := s.muted.Render("Plain text API tests. No magic.")

	var sub string
	if m.workspace.HasConfig {
		sub = s.success.Render("✓ project ready") + s.muted.Render(" — no .http files here yet")
	} else {
		sub = s.muted.Render("This folder has no hitspec project yet.")
	}

	action := func(k, desc string) string {
		return s.tag.Render(k) + "  " + desc
	}
	actions := lipgloss.JoinVertical(lipgloss.Left,
		action("g", "generate a sample project"),
		action("n", "new request file"),
		action("7", "import from curl · OpenAPI · Postman"),
		action("ctrl+p", "command palette"),
		action("?", "keyboard help"),
	)

	body := lipgloss.JoinVertical(lipgloss.Center,
		title,
		tagline,
		"",
		sub,
		"",
		actions,
	)
	card := s.panelHot.Padding(1, 4).Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, card)
}

// scaffoldSampleCmd writes a starter project (config + example request file) and
// reloads the workspace so the welcome card gives way to a populated file list.
func scaffoldSampleCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		created, err := mgr.ScaffoldSample(ctx)
		if err != nil {
			return simpleMsg{kind: "generate sample", err: err}
		}
		kind := "created sample project"
		if len(created) == 1 {
			kind = "created " + created[0]
		}
		return simpleMsg{kind: kind}
	}
}

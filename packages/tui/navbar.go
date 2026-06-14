package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// navstrip renders a full-width strip of numbered screen tabs with the active
// screen highlighted, making the 1–9 screen switching discoverable at a glance.
// On terminals too narrow for the full strip it collapses to a compact
// position indicator.
func (m model) navstrip() string {
	s := m.styles
	items := make([]string, len(screenNames))
	for i, name := range screenNames {
		num := i + 1
		if screen(i) == m.screen {
			items[i] = s.navActive.Render(fmt.Sprintf("%d %s", num, name))
		} else {
			items[i] = s.navNum.Render(strconv.Itoa(num)) + s.navItem.Render(" "+name)
		}
	}
	sep := s.navItem.Render("  ")
	full := strings.Join(items, sep)
	if lipgloss.Width(full) <= m.width-2 {
		return s.navBar.Width(m.width).Render(full)
	}

	// Compact fallback: active screen + position + a hint to the number keys.
	compact := s.navActive.Render(fmt.Sprintf("%d/%d %s", int(m.screen)+1, len(screenNames), screenNames[m.screen])) +
		s.navItem.Render("   1-9 switch")
	return s.navBar.Width(m.width).Render(compact)
}

// hints returns a concise, context-aware action summary for the status bar. It
// reflects the current screen and focused pane (and edit/modal state) so the
// most relevant keys are always shown instead of a fixed, generic list.
func (m model) hints() string {
	s := m.styles
	var h string
	switch {
	case m.editing && m.focus == focusSource:
		h = "ctrl+s save · esc cancel"
	case m.screen == screenWorkspace:
		switch m.focus {
		case focusFiles:
			h = "enter open · e edit · n new · D delete"
		case focusRequests:
			h = "↑↓ rows · r run · R run file"
		case focusSource:
			h = "e edit · ctrl+s save · r run · R run file"
		case focusResponse:
			h = "←→/[] tabs · r run · R run file"
		default:
			h = "tab next pane · r run"
		}
	case m.screen == screenHistory:
		h = "enter details · D delete · ctrl+r refresh"
	default:
		h = "e fields · enter run · esc back"
	}
	return s.help.Render(h+" · ") + s.accent.Render("?") + s.help.Render(" help · ") + s.accent.Render("ctrl+p") + s.help.Render(" palette")
}

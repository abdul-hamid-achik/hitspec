package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// helpSection is a titled group of key bindings rendered as one column block.
type helpSection struct {
	title string
	keys  []key.Binding
}

// helpOverlay renders a centered, sectioned keyboard reference. It replaces the
// old single-line FullHelpView crammed into the status bar, which truncated on
// every realistic terminal width.
func (m model) helpOverlay() string {
	s := m.styles
	sections := []helpSection{
		{"Screens", []key.Binding{
			m.keys.Workspace, m.keys.Stress, m.keys.Mock, m.keys.Contract,
			m.keys.Record, m.keys.History, m.keys.Import, m.keys.Cookies, m.keys.Settings,
		}},
		{"Workspace", []key.Binding{
			m.keys.Open, m.keys.Edit, m.keys.Save, m.keys.RunRequest, m.keys.RunFile,
			m.keys.Refresh, m.keys.NewFile, m.keys.GenerateSample, m.keys.DeleteFile,
		}},
		{"Global", []key.Binding{
			m.keys.Palette, m.keys.EnvSwitch, m.keys.ThemeSwitch, m.keys.Search, m.keys.Tab,
			m.keys.BackTab, m.keys.Cancel, m.keys.Help, m.keys.Quit,
		}},
	}

	// Align the key column within each section for a tidy, scannable layout.
	cols := make([]string, 0, len(sections))
	for _, sec := range sections {
		keyW := 0
		for _, b := range sec.keys {
			if w := lipgloss.Width(b.Help().Key); w > keyW {
				keyW = w
			}
		}
		lines := []string{s.title.Render(sec.title), ""}
		for _, b := range sec.keys {
			h := b.Help()
			gap := strings.Repeat(" ", max(1, keyW-lipgloss.Width(h.Key)+2))
			lines = append(lines, s.accent.Render(h.Key)+gap+s.muted.Render(h.Desc))
		}
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left, lines...))
	}

	// Separate the columns with padded gutters.
	spaced := make([]string, 0, len(cols)*2-1)
	for i, c := range cols {
		if i > 0 {
			spaced = append(spaced, "   ")
		}
		spaced = append(spaced, c)
	}
	grid := lipgloss.JoinHorizontal(lipgloss.Top, spaced...)

	header := s.accent.Render("Keyboard shortcuts")
	footer := s.muted.Render("press any key to close")
	body := lipgloss.JoinVertical(lipgloss.Left, header, "", grid, "", footer)
	return s.panelHot.Padding(1, 3).Render(body)
}

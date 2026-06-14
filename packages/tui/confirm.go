package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// confirmState is a modal yes/no prompt gating a destructive action. action is
// the command run when the user confirms.
type confirmState struct {
	title  string
	body   string
	action tea.Cmd
}

// view renders the confirm dialog box.
func (c *confirmState) view(s styles) string {
	title := s.danger.Bold(true).Render(c.title)
	body := s.muted.Render(c.body)
	hint := s.help.Render("y / enter  confirm      n / esc  cancel")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", hint)
	return s.panelHot.Padding(1, 2).Render(content)
}

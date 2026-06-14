package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// promptState is a modal single-line text input (e.g. rename/duplicate/save-as).
// onSubmit receives the entered value and returns the command to run.
type promptState struct {
	title    string
	input    textinput.Model
	onSubmit func(value string) tea.Cmd
}

func newPrompt(title, placeholder, initial string, onSubmit func(string) tea.Cmd) *promptState {
	in := textinput.New()
	in.Placeholder = placeholder
	in.CharLimit = 512
	in.SetWidth(50)
	in.SetValue(initial)
	in.Focus()
	return &promptState{title: title, input: in, onSubmit: onSubmit}
}

func (p *promptState) view(s styles) string {
	title := s.title.Render(p.title)
	hint := s.help.Render("enter  confirm      esc  cancel")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", p.input.View(), "", hint)
	return s.panelHot.Padding(1, 2).Render(content)
}

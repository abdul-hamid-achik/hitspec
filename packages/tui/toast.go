package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// toastSeverity controls a toast's icon and accent color.
type toastSeverity int

const (
	toastInfo toastSeverity = iota
	toastSuccess
	toastWarn
	toastError
)

const (
	toastTTL     = 4 * time.Second
	toastMax     = 3
	toastMaxText = 44
)

// toast is a single transient notification.
type toast struct {
	id       int
	text     string
	severity toastSeverity
}

// toastCenter is an immutable-style queue of toasts. Mutating methods return a
// new value so it composes naturally with Bubble Tea's value-receiver Update.
type toastCenter struct {
	items  []toast
	nextID int
}

// toastExpiredMsg removes a toast once its TTL elapses.
type toastExpiredMsg struct{ id int }

func newToastCenter() toastCenter { return toastCenter{} }

// push appends a toast (dropping the oldest beyond toastMax) and returns the
// updated center plus a command that expires the new toast after toastTTL.
func (c toastCenter) push(severity toastSeverity, text string) (toastCenter, tea.Cmd) {
	c.nextID++
	id := c.nextID
	items := append([]toast{}, c.items...)
	items = append(items, toast{id: id, text: truncateText(text, toastMaxText), severity: severity})
	if len(items) > toastMax {
		items = items[len(items)-toastMax:]
	}
	c.items = items
	cmd := tea.Tick(toastTTL, func(time.Time) tea.Msg { return toastExpiredMsg{id: id} })
	return c, cmd
}

// expire drops the toast with the given id.
func (c toastCenter) expire(id int) toastCenter {
	items := make([]toast, 0, len(c.items))
	for _, t := range c.items {
		if t.id != id {
			items = append(items, t)
		}
	}
	c.items = items
	return c
}

func (c toastCenter) empty() bool { return len(c.items) == 0 }

// view stacks the active toasts (newest last) right-aligned.
func (c toastCenter) view(s styles) string {
	if len(c.items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(c.items))
	for _, t := range c.items {
		lines = append(lines, t.render(s))
	}
	return lipgloss.JoinVertical(lipgloss.Right, lines...)
}

func (t toast) render(s styles) string {
	// Colors track the active theme by reading the semantic styles.
	icon, sev := "i", s.info
	switch t.severity {
	case toastSuccess:
		icon, sev = "✓", s.success
	case toastWarn:
		icon, sev = "!", s.warn
	case toastError:
		icon, sev = "✗", s.danger
	case toastInfo:
		icon, sev = "i", s.info
	}
	color := sev.GetForeground()
	marker := lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon)
	body := marker + " " + t.text
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Background(s.status.GetBackground()).
		Foreground(s.title.GetForeground()).
		Padding(0, 1).
		Render(body)
}

// truncateText shortens s to at most n characters with an ellipsis, counting by
// rune so multibyte text is not split mid-character.
func truncateText(s string, n int) string {
	if n <= 1 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

package tui

import (
	"charm.land/bubbles/v2/list"
)

// themeItem is a selectable theme in the picker overlay.
type themeItem struct {
	name   string
	active bool
}

func (i themeItem) FilterValue() string { return i.name }

func (i themeItem) Title() string {
	if i.active {
		return "● " + i.name
	}
	return "  " + i.name
}

func (i themeItem) Description() string { return "color theme" }

// buildThemeItems lists every registered theme, marking the active one.
func buildThemeItems(active string) []list.Item {
	items := make([]list.Item, 0, len(themes))
	for _, p := range themes {
		items = append(items, themeItem{name: p.name, active: p.name == active})
	}
	return items
}

// applyTheme switches the live color theme, re-deriving every style and
// re-theming the components that cache styles (response viewer, spinner). Unknown
// names are ignored so a bad value can never blank the UI.
func (m *model) applyTheme(name string) {
	p, ok := paletteByName(name)
	if !ok {
		return
	}
	m.theme = p.name
	m.styles = newStyles(p)
	m.respView.setStyles(m.styles)
	m.spinner.Style = m.styles.accent
	m.resize()
}

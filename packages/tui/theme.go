package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// palette is a theme's semantic color set. Styles are derived from these slots
// in newStyles, so adding a theme is just adding a palette to the registry.
type palette struct {
	name string

	bg     color.Color // base background
	bgAlt  color.Color // bars (top/status/nav)
	bgSel  color.Color // selection background
	muted  color.Color // borders + dim/help text
	fg     color.Color // primary text
	bright color.Color // titles / emphasis text
	accent color.Color // active border, accents, badges

	success color.Color
	warning color.Color
	danger  color.Color
	info    color.Color

	// HTTP method badge colors.
	get   color.Color
	post  color.Color
	put   color.Color
	del   color.Color
	patch color.Color
}

// themes is the ordered theme registry. The first entry is the default.
var themes = []palette{
	{
		name:    "Nord",
		bg:      lipgloss.Color("#2E3440"),
		bgAlt:   lipgloss.Color("#3B4252"),
		bgSel:   lipgloss.Color("#434C5E"),
		muted:   lipgloss.Color("#4C566A"),
		fg:      lipgloss.Color("#D8DEE9"),
		bright:  lipgloss.Color("#ECEFF4"),
		accent:  lipgloss.Color("#88C0D0"),
		success: lipgloss.Color("#A3BE8C"),
		warning: lipgloss.Color("#EBCB8B"),
		danger:  lipgloss.Color("#BF616A"),
		info:    lipgloss.Color("#81A1C1"),
		get:     lipgloss.Color("#A3BE8C"),
		post:    lipgloss.Color("#EBCB8B"),
		put:     lipgloss.Color("#81A1C1"),
		del:     lipgloss.Color("#BF616A"),
		patch:   lipgloss.Color("#B48EAD"),
	},
	{
		name:    "Catppuccin Mocha",
		bg:      lipgloss.Color("#1E1E2E"),
		bgAlt:   lipgloss.Color("#313244"),
		bgSel:   lipgloss.Color("#45475A"),
		muted:   lipgloss.Color("#6C7086"),
		fg:      lipgloss.Color("#CDD6F4"),
		bright:  lipgloss.Color("#F5E0DC"),
		accent:  lipgloss.Color("#89B4FA"),
		success: lipgloss.Color("#A6E3A1"),
		warning: lipgloss.Color("#F9E2AF"),
		danger:  lipgloss.Color("#F38BA8"),
		info:    lipgloss.Color("#89DCEB"),
		get:     lipgloss.Color("#A6E3A1"),
		post:    lipgloss.Color("#F9E2AF"),
		put:     lipgloss.Color("#89B4FA"),
		del:     lipgloss.Color("#F38BA8"),
		patch:   lipgloss.Color("#CBA6F7"),
	},
	{
		name:    "Dracula",
		bg:      lipgloss.Color("#282A36"),
		bgAlt:   lipgloss.Color("#343746"),
		bgSel:   lipgloss.Color("#44475A"),
		muted:   lipgloss.Color("#6272A4"),
		fg:      lipgloss.Color("#F8F8F2"),
		bright:  lipgloss.Color("#FFFFFF"),
		accent:  lipgloss.Color("#BD93F9"),
		success: lipgloss.Color("#50FA7B"),
		warning: lipgloss.Color("#F1FA8C"),
		danger:  lipgloss.Color("#FF5555"),
		info:    lipgloss.Color("#8BE9FD"),
		get:     lipgloss.Color("#50FA7B"),
		post:    lipgloss.Color("#F1FA8C"),
		put:     lipgloss.Color("#8BE9FD"),
		del:     lipgloss.Color("#FF5555"),
		patch:   lipgloss.Color("#FF79C6"),
	},
	{
		name:    "Tokyo Night",
		bg:      lipgloss.Color("#1A1B26"),
		bgAlt:   lipgloss.Color("#24283B"),
		bgSel:   lipgloss.Color("#2F3549"),
		muted:   lipgloss.Color("#565F89"),
		fg:      lipgloss.Color("#C0CAF5"),
		bright:  lipgloss.Color("#FFFFFF"),
		accent:  lipgloss.Color("#7AA2F7"),
		success: lipgloss.Color("#9ECE6A"),
		warning: lipgloss.Color("#E0AF68"),
		danger:  lipgloss.Color("#F7768E"),
		info:    lipgloss.Color("#7DCFFF"),
		get:     lipgloss.Color("#9ECE6A"),
		post:    lipgloss.Color("#E0AF68"),
		put:     lipgloss.Color("#7AA2F7"),
		del:     lipgloss.Color("#F7768E"),
		patch:   lipgloss.Color("#BB9AF7"),
	},
	{
		name:    "Gruvbox Dark",
		bg:      lipgloss.Color("#282828"),
		bgAlt:   lipgloss.Color("#3C3836"),
		bgSel:   lipgloss.Color("#504945"),
		muted:   lipgloss.Color("#928374"),
		fg:      lipgloss.Color("#EBDBB2"),
		bright:  lipgloss.Color("#FBF1C7"),
		accent:  lipgloss.Color("#83A598"),
		success: lipgloss.Color("#B8BB26"),
		warning: lipgloss.Color("#FABD2F"),
		danger:  lipgloss.Color("#FB4934"),
		info:    lipgloss.Color("#8EC07C"),
		get:     lipgloss.Color("#B8BB26"),
		post:    lipgloss.Color("#FABD2F"),
		put:     lipgloss.Color("#83A598"),
		del:     lipgloss.Color("#FB4934"),
		patch:   lipgloss.Color("#D3869B"),
	},
}

// defaultPalette is the theme used when none is requested or a name is unknown.
func defaultPalette() palette { return themes[0] }

// paletteByName looks up a theme case-insensitively (so "nord", "Nord", and
// "tokyo night" all resolve). The second return is false when not found.
func paletteByName(name string) (palette, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, p := range themes {
		if strings.ToLower(p.name) == want {
			return p, true
		}
	}
	return palette{}, false
}

type styles struct {
	app       lipgloss.Style
	top       lipgloss.Style
	status    lipgloss.Style
	panel     lipgloss.Style
	panelHot  lipgloss.Style
	title     lipgloss.Style
	muted     lipgloss.Style
	success   lipgloss.Style
	warn      lipgloss.Style
	danger    lipgloss.Style
	info      lipgloss.Style
	accent    lipgloss.Style
	tag       lipgloss.Style
	help      lipgloss.Style
	source    lipgloss.Style
	selection lipgloss.Style
	navBar    lipgloss.Style
	navActive lipgloss.Style
	navItem   lipgloss.Style
	navNum    lipgloss.Style

	// methodBadges maps an uppercased HTTP method to its badge style.
	methodBadges map[string]lipgloss.Style
}

// methodBadge returns the badge style for an HTTP method, falling back to the
// generic accent tag for unknown/custom methods.
func (s styles) methodBadge(method string) lipgloss.Style {
	if st, ok := s.methodBadges[strings.ToUpper(strings.TrimSpace(method))]; ok {
		return st
	}
	return s.tag
}

func newStyles(p palette) styles {
	badge := func(c color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(p.bg).Background(c).Bold(true).Padding(0, 1)
	}
	return styles{
		app: lipgloss.NewStyle().
			Foreground(p.fg).
			Background(p.bg),
		top: lipgloss.NewStyle().
			Foreground(p.bright).
			Background(p.bgAlt).
			Padding(0, 1).
			Bold(true),
		status: lipgloss.NewStyle().
			Foreground(p.fg).
			Background(p.bgAlt).
			Padding(0, 1),
		panel: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(p.muted).
			Padding(0, 1),
		panelHot: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(p.accent).
			Padding(0, 1),
		title: lipgloss.NewStyle().
			Foreground(p.bright).
			Bold(true),
		muted: lipgloss.NewStyle().
			Foreground(p.muted),
		success: lipgloss.NewStyle().
			Foreground(p.success),
		warn: lipgloss.NewStyle().
			Foreground(p.warning),
		danger: lipgloss.NewStyle().
			Foreground(p.danger),
		info: lipgloss.NewStyle().
			Foreground(p.info),
		accent: lipgloss.NewStyle().
			Foreground(p.accent).
			Bold(true),
		tag: lipgloss.NewStyle().
			Foreground(p.bg).
			Background(p.accent).
			Padding(0, 1),
		help: lipgloss.NewStyle().
			Foreground(p.muted),
		source: lipgloss.NewStyle().
			Foreground(p.fg).
			Background(p.bg),
		selection: lipgloss.NewStyle().
			Foreground(p.bright).
			Background(p.bgSel),
		navBar: lipgloss.NewStyle().
			Background(p.bg).
			Padding(0, 1),
		navActive: lipgloss.NewStyle().
			Foreground(p.bg).
			Background(p.accent).
			Bold(true).
			Padding(0, 1),
		navItem: lipgloss.NewStyle().
			Foreground(p.muted),
		navNum: lipgloss.NewStyle().
			Foreground(p.accent).
			Bold(true),
		methodBadges: map[string]lipgloss.Style{
			"GET":     badge(p.get),
			"POST":    badge(p.post),
			"PUT":     badge(p.put),
			"DELETE":  badge(p.del),
			"PATCH":   badge(p.patch),
			"HEAD":    badge(p.info),
			"OPTIONS": badge(p.muted),
		},
	}
}

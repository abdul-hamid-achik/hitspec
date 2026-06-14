package tui

import "charm.land/lipgloss/v2"

// overlayBox composites box onto base at absolute cell (x, y) using lipgloss v2's
// layer compositor, preserving whatever is underneath. base is expected to
// already fill w×h.
func overlayBox(base string, w, h int, box string, x, y int) string {
	if w <= 0 || h <= 0 || box == "" {
		return base
	}
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	)
	canvas := lipgloss.NewCanvas(w, h)
	canvas.Compose(comp)
	return canvas.Render()
}

// overlayBottomRight floats box in the lower-right corner, leaving a one-cell
// right margin and two rows for the status bar.
func overlayBottomRight(base string, w, h int, box string) string {
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	return overlayBox(base, w, h, box, max(0, w-bw-1), max(0, h-bh-2))
}

// overlayCenter floats box in the middle of the base, preserving the background
// (unlike lipgloss.Place which blanks it).
func overlayCenter(base string, w, h int, box string) string {
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	return overlayBox(base, w, h, box, max(0, (w-bw)/2), max(0, (h-bh)/2))
}

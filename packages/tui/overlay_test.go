package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestOverlayBoxPreservesBackground(t *testing.T) {
	w, h := 40, 10
	base := lipgloss.NewStyle().Width(w).Height(h).Render("BASEMARK")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render("BOX")

	out := overlayBox(base, w, h, box, 5, 2)
	if lipgloss.Height(out) != h {
		t.Fatalf("overlay height = %d, want %d", lipgloss.Height(out), h)
	}
	if !strings.Contains(out, "BASEMARK") {
		t.Fatalf("overlay dropped the background:\n%s", out)
	}
	if !strings.Contains(out, "BOX") {
		t.Fatalf("overlay did not draw the box:\n%s", out)
	}
}

func TestOverlayBoxNoops(t *testing.T) {
	base := "unchanged"
	if got := overlayBox(base, 0, 0, "x", 0, 0); got != base {
		t.Fatalf("zero-size overlay should be a no-op, got %q", got)
	}
	if got := overlayBox(base, 10, 10, "", 0, 0); got != base {
		t.Fatalf("empty-box overlay should be a no-op, got %q", got)
	}
}
